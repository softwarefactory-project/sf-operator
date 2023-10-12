# Copyright (C) 2023 Red Hat
# SPDX-License-Identifier: Apache-2.0

#!/usr/bin/python


import re
import argparse
import yaml
import math


prog_desc = """
This utility scrapes the Zuul or Nodepool documentation to create a statsd exporter mapping config.

This should be run after each new Zuul release to keep up to date.
"""


class quoted(str): pass


def represent_with_quotes(dumper, data):
    return dumper.represent_scalar('tag:yaml.org,2002:str', data, style='"')


yaml.add_representer(quoted, represent_with_quotes)


# some labels in the docs have forbidden characters in them
def fix_label(label):
    if label == "queue name":
        return "queue"
    elif label == "connection-name":
        return "connection"
    elif label == "image name":
        return "image"
    elif label == "provider name":
        return "provider"
    elif label == "diskimage_name":
        return "diskimage"
    else:
        return label


def stat_to_mapping(stat):
    match = ''
    name = ''
    labels = {}
    label_count = 1
    for k in stat:
        if k.startswith('<'):
            match += '*.'
            labels[fix_label(k[1:-1])] = quoted('$%i' % label_count)
            label_count +=1
        else:
            match += k + '.'
            name += k + '_'
    help_msg = ""
    if stat[0].startswith('zuul'):
        help_msg = 'Description at https://zuul-ci.org/docs/zuul/latest/monitoring.html#stat-%s' % '.'.join(stat)
    else:
        help_msg = 'Description at https://zuul-ci.org/docs/nodepool/latest/operation.html#stat-%s' % '.'.join(stat)
    # explicit result values for launcher-type metrics, to make it easier to parse and query.
    if stat[0].startswith('nodepool') and 'result' in labels:
        labels_ready = dict((lk, labels[lk]) for lk in labels if lk != 'result')
        mapping_ready = {
            'match': match[:-2] + 'ready',
            'name': name[:-1] + '_ready',
            'help': help_msg,
            'labels': labels_ready,
        }
        labels_error = dict((lk, labels[lk]) for lk in labels if lk != 'result')
        labels_error['error'] = labels['result']
        mapping_error = {
            'match': match[:-2] + 'error.*',
            'name': name[:-1] + '_error',
            'help': help_msg,
            'labels': labels_error,
        }
        return [mapping_ready, mapping_error, ]
    else:
        mapping = {
            'match': match[:-1],
            'name': name[:-1],
            'help': help_msg,
        }
        if labels:
            mapping['labels'] = labels
        return [mapping, ]


docstat_re = re.compile('^(\s*)\.\. (zuul:)?stat:: (.+)$')


if __name__ == "__main__":

    parser = argparse.ArgumentParser(
        prog='zuuldoc2statsdmapper',
        description=prog_desc,
    )
    parser.add_argument('-i', '--input', metavar='path/to/zuul/doc/source/monitoring.rst')
    parser.add_argument('output_file', default='statsd_mapping.yaml')

    mappings = []
    args = parser.parse_args()
    with open(args.input) as zuuldoc:
        stat_chain = []
        current_indent = 0
        for l in zuuldoc.readlines():
            # Beyond this paragraph in Zuul's documentation, there aren't any metrics anymore
            if "Prometheus monitoring" in l:
                break
            m = docstat_re.match(l)
            if m:
                indent, stat = math.ceil(len(m.groups()[0]) / 3), m.groups()[2].split('.')
                if indent == 0:
                    stat_chain = [stat,]
                else:
                    if indent > current_indent:
                        stat_chain.append(stat)
                    elif indent == current_indent:
                        stat_chain = stat_chain[:-1]
                        stat_chain.append(stat)
                    else:
                        diff = current_indent - indent + 1
                        stat_chain = stat_chain[:-diff]
                        stat_chain.append(stat)
                current_indent = indent
                mapping = stat_to_mapping(sum(stat_chain, []))
                mappings += mapping

    # Nodepool: Append OpenStack API metrics issued by openstacksdk
    # if stat_chain[0][0].startswith('nodepool'):
    #     mappings.append(
    #         {
    #             'match': 'openstack.api.*.*.*.*',
    #             'name': 'openstack_api',
    #             'help': 'Description at https://zuul-ci.org/docs/nodepool/latest/operation.html#openstack-api-metrics',
    #             'labels': {
    #                 'service': quoted('$1'),
    #                 'method': quoted('$2'),
    #                 'operation': quoted('$3'),
    #                 'status': quoted('$4'),
    #             }
    #         }
    #     )

    # Drop all non-matching metrics to avoid spamming
    mappings.append(
        {
         'match': '.',
         'match_type': 'regex',
         'action': 'drop',
         'name': quoted('dropped')
        }
    )
    with open(args.output_file, 'w') as o:
        o.write("# Auto-generated with zuuldoc2statsdmapper.py, please check with: \n")
        o.write("# podman run --rm -v %s:/tmp/statsd_mapping.yaml:z docker.io/prom/statsd-exporter --statsd.mapping-config=/tmp/statsd_mapping.yaml\n#\n" % args.output_file)
        o.write(yaml.dump({'mappings': mappings}))

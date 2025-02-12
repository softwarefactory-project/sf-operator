#!/bin/env pythons.
# Copyright (C) 2025 Red Hat
# SPDX-License-Identifier: Apache-2.0

import configparser, json, sys, yaml


def read_connections(zuul_conf):
    """Read connections from zuul.conf"""
    parser = configparser.ConfigParser()
    parser.read_string(zuul_conf)
    connections = {}
    for section in parser.sections():
        kv = section.split()
        if kv[0] == "connection":
            if parser.has_option(section, "baseurl"):
                url = parser.get(section, "baseurl").rstrip("/")
            elif section == "connection softwarefactory-project.io":
                url = "https://softwarefactory-project.io/r"
            else:
                url = ""
            # Get the connection name, driver and baseurl
            connections[kv[1]] = dict(driver=parser.get(section, "driver"), baseurl=url)
    return connections


test_zuul_conf = """
[merger]
[connection opendev.org]
driver              = gerrit
baseurl             = https://gerrit.sfop.me
[connection gitlab.com]
driver              = gitlab
baseurl             = https://gitlab.com
[connection github.com]
driver              = github
"""


def read_repos(zuul_yaml):
    """Read repositories from zuul main.yaml"""
    tenants = yaml.safe_load(zuul_yaml)
    projs = []
    for tenant in tenants:
        if not tenant.get("tenant"):
            continue
        for conn, conf in tenant["tenant"].get("source", {}).items():
            for proj in conf.get("config-projects", []) + conf.get(
                "untrusted-projects", []
            ):
                # TODO: add support for project group
                if isinstance(proj, str):
                    # This is a literal project, assume default branch name
                    projs.append((conn, proj, "master"))
                else:
                    # This is a project object, it's name is the first key
                    name = list(proj.keys())[0]
                    branch = proj.get("load-branch", "master")
                    projs.append((conn, name, branch))

    return projs


test_zuul_yaml = """
- tenant:
    name: demo-tenant
    source:
        gitlab.com:
            config-projects:
                - demo-tenant-config
            untrusted-projects:
                - demo-project
        opendev.org:
            config-projects:
                - zuul/sandbox-config:
                    load-branch: main
            untrusted-projects:
                - zuul/zuul-jobs
"""


def get_git_urls(conn, repo, branch):
    """Create the hound URLs from the zuul connection and repo config."""
    base_url = conn["baseurl"]
    if conn["driver"] == "gerrit":
        uri = base_url + f"/{repo}"
        gitweb = (
            base_url
            + f"/plugins/gitiles/{repo}/+/refs/head/{branch}"
            + "/{path}{anchor}"
        )
        anchor = "#{line}"
        if "https://review.gerrithub.io" in base_url:
            gitweb = f"http://github.com/{repo}/blob/{branch}/" + "{path}{anchor}"
            anchor = "#L{line}"
    elif conn["driver"] == "github":
        uri = f"http://github.com/{repo}"
        gitweb = f"http://github.com/{repo}/blob/{branch}/" + "{path}{anchor}"
        anchor = "#L{line}"
    elif conn["driver"] == "pagure":
        uri = base_url + f"/{repo}"
        gitweb = base_url + f"/{repo}/blob/{branch}/f/" + "{path}{anchor}"
        anchor = "#_{line}"
    elif conn["driver"] == "gitlab":
        uri = base_url + f"/{repo}"
        gitweb = base_url + f"/{repo}/-/blob/{branch}/" + "{path}{anchor}"
        anchor = "#L{line}"
    elif conn["driver"] == "git" and base_url.startswith("https://opendev.org"):
        uri = base_url + f"/{repo}"
        gitweb = base_url + f"/{repo}/src/branch/{branch}/" + "{path}{anchor}"
        anchor = "#L{line}"
    else:
        return None, None, None
    return uri, gitweb, anchor


def render_hound(connections, projs):
    """Create the hound-search config"""
    repos = {}
    for conn, repo, branch in projs:
        url, base_url, anchor = get_git_urls(connections[conn], repo, branch)
        if not url:
            continue
        repos[repo] = {
            "url": url,
            "ms-between-poll": int(12 * 60 * 60 * 1000),
            "vcs-connfig": dict(ref=branch),
            "url-pattern": {
                "base-url": base_url,
                "anchor": anchor,
            },
        }
    return {
        "max-concurrent-indexers": 2,
        "dbpath": "/var/lib/hound/data",
        "repos": repos,
    }


def do_main():
    try:
        zuul_yaml = open("/var/lib/hound/config/zuul/main.yaml").read()
    except:
        zuul_yaml = "[]"
    conf = json.dumps(
        render_hound(
            read_connections(open("/etc/zuul/zuul.conf").read()),
            read_repos(zuul_yaml),
        )
    )
    open("/var/lib/hound/config.json", "w").write(conf)


def do_test():
    conf = render_hound(read_connections(test_zuul_conf), read_repos(test_zuul_yaml))
    expected = {
        "max-concurrent-indexers": 2,
        "dbpath": "/var/lib/hound/data",
        "repos": {
            "demo-tenant-config": {
                "url": "https://gitlab.com/demo-tenant-config",
                "ms-between-poll": 43200000,
                "vcs-connfig": {"ref": "master"},
                "url-pattern": {
                    "base-url": "https://gitlab.com/demo-tenant-config/-/blob/master/{path}{anchor}",
                    "anchor": "#L{line}",
                },
            },
            "demo-project": {
                "url": "https://gitlab.com/demo-project",
                "ms-between-poll": 43200000,
                "vcs-connfig": {"ref": "master"},
                "url-pattern": {
                    "base-url": "https://gitlab.com/demo-project/-/blob/master/{path}{anchor}",
                    "anchor": "#L{line}",
                },
            },
            "zuul/sandbox-config": {
                "url": "https://gerrit.sfop.me/zuul/sandbox-config",
                "ms-between-poll": 43200000,
                "vcs-connfig": {"ref": "master"},
                "url-pattern": {
                    "base-url": "https://gerrit.sfop.me/plugins/gitiles/zuul/sandbox-config/+/refs/head/master/{path}{anchor}",
                    "anchor": "#{line}",
                },
            },
            "zuul/zuul-jobs": {
                "url": "https://gerrit.sfop.me/zuul/zuul-jobs",
                "ms-between-poll": 43200000,
                "vcs-connfig": {"ref": "master"},
                "url-pattern": {
                    "base-url": "https://gerrit.sfop.me/plugins/gitiles/zuul/zuul-jobs/+/refs/head/master/{path}{anchor}",
                    "anchor": "#{line}",
                },
            },
        },
    }
    if conf != expected:
        print("Bad config:")
        print(conf)


if __name__ == "__main__":
    if "test" in sys.argv:
        do_test()
    else:
        do_main()

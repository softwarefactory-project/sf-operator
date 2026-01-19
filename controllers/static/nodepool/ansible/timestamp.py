import datetime

from ansible import constants as C
from ansible import context
from ansible import __version__ as ansiblecore_version
from ansible.plugins.callback import default, CallbackBase
import ansible.utils.display as dsplay


"""
Timestamped output callback plugin

This simple plugin will prefix ansible-playbook's stdout with human-readable
timestamps. This also makes fluent bit able to timestamp events properly,
so that build logs can be queried/displayed in order.

MAKE SURE THAT THIS MODULE MATCHES ANSIBLE-CORE'S VERSION SHIPPED ON THE
NODEPOOL-BUILDER CONTAINER! See https://softwarefactory-project.io/r/plugins/gitiles/containers/+/refs/heads/master/images-sf/master/versions.dhall


It is based on the [default callback plugin](https://github.com/ansible/ansible/blob/2.16/lib/ansible/plugins/callback/default.py)
"""


def ts(msg):
    ts_msg = []
    for l in msg.split('\n'):
        now = datetime.datetime.now()
        ts_msg.append(u"{now} | {line}".format(now=now, line=l))
    return '\n'.join(ts_msg)


class TimestampedDisplay(dsplay.Display):

    @dsplay.proxy_display
    def display(
        self,
        msg,
        color=None,
        stderr=False,
        screen_only=False,
        log_only=False,
        newline=True,
    ):
        super().display(
            ts(msg), color, stderr, screen_only, log_only, newline
        )

    def banner(self, msg, color=None, cows=True):
        # we don't care about cowsay, only use stars.
        msg = msg.strip()
        star_len = self.columns - (len(msg) + 3 + len("{now}".format(now=datetime.datetime.now())))
        if star_len <= 3:
            star_len = 3
        stars = u"*" * star_len
        self.display(u"%s %s" % (msg, stars), color=color)



class CallbackModule(default.CallbackModule):
    '''
    Simple output callback that appends a timestamp in front of
    each line of ansible-playbook's output.

    Other than that, it behaves exactly like the "default" callback plugin.
    '''

    CALLBACK_VERSION = 2.0
    ANSIBLE_VERSION = "2.16"
    CALLBACK_TYPE = 'stdout'
    CALLBACK_NAME = 'timestamp_output'

    def __init__(self):
        if not ansiblecore_version.startswith(self.ANSIBLE_VERSION):
            raise Exception(
                "Timestamp callback module is not compatible with the installed "
                "version of ansible-core ({ac}), expected: {av}".format(ac=ansiblecore_version, av=self.ANSIBLE_VERSION)
            )
        self._play = None
        self._last_task_banner = None
        self._last_task_name = None
        self._task_type_cache = {}
        super(default.CallbackModule, self).__init__(display=TimestampedDisplay())

    # Override plugin options with default's ... default values
    def get_option(self, k):
        options = {
            'display_skipped_hosts': True,
            'display_ok_hosts': True,
            'display_failed_stderr': False,
            'show_custom_stats': False,
            'show_per_host_start': False,
            'check_mode_markers': False,
            'show_task_path_on_failure': False,
        }
        return options[k]

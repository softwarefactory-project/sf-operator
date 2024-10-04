# Copyright (C) 2023 Red Hat
# SPDX-License-Identifier: Apache-2.0


import os
import logging

import requests


class SimpleFluentBitHTTPInputHandler(logging.Handler):
    """A minimal handler for sending logs to the HTTP Input
    of a Fluent Bit collector."""
    def __init__(self, url, env_prefix=None):
        logging.Handler.__init__(self)
        self.url = url
        self.env_prefix = env_prefix

    def emit(self, record):
        d = {
            'log': self.format(record)
        }
        if self.env_prefix is not None:
            for envvar in os.environ:
                if envvar.startswith(self.env_prefix):
                    key = envvar[len(self.env_prefix):].lower()
                    d[key] = os.environ[envvar]
        try:
            req = requests.post(self.url, json=d)
            req.raise_for_status()
        except requests.HTTPError as e:
            self.handleError(record)
# Copyright (C) 2022 Red Hat
# SPDX-License-Identifier: Apache-2.0

from setuptools import setup

setup(
    name="sf_operator",
    setup_requires=["setuptools_scm"],
    packages=["sf_operator"],
    use_scm_version=True,
    description="Software Factory operator library",
    long_description=open("README.md").read(),
    long_description_content_type="text/markdown",
    entry_points={"console_scripts": ["sf-operator=sf_operator.main:main"]},
    install_requires=["pynotedb", "managesf"],
)

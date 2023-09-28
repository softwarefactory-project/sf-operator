#!/usr/bin/env python3
# Licensed under the Apache License, Version 2.0 (the "License"); you may
# not use this file except in compliance with the License. You may obtain
# a copy of the License at
#
#      http://www.apache.org/licenses/LICENSE-2.0
#
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS, WITHOUT
# WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the
# License for the specific language governing permissions and limitations
# under the License.


import argparse
import subprocess
import os


# Wrapper to be called by nodepool-builder to start ansible-playbook

# To call it outside of nodepool-builder to debug images
# run: dib-ansible -t qcow2 -o /tmp/test ./fedora-30-cloud.yaml


def main():
    # Fake dib interface
    parser = argparse.ArgumentParser()
    parser.add_argument("-x", action='store_true', help="noop")
    parser.add_argument("-t", help="Image types")
    parser.add_argument("--checksum", action='store_true', help="noop")
    parser.add_argument("--no-tmpfs", action='store_true', help="noop")
    parser.add_argument("--qemu-img-options", help="noop")
    parser.add_argument("-o", help="Image output")
    parser.add_argument("playbook", help="noop")
    args = parser.parse_args()
    cmd = ["/usr/bin/ansible-playbook", "-v"]

    playbook = args.playbook
    playbook_path = None

    if os.path.exists(playbook):
        playbook_path = playbook
    else:
        vp = os.path.join(
            "/var/lib/nodepool/config/nodepool/dib-ansible", playbook)
        if os.path.exists(vp):
            playbook_path = vp
    if not playbook_path:
        print("Can't find playbook %s" % playbook)
        exit(1)

    cmd.append(playbook_path)

    # Set the image output var
    cmd.extend(["-e", "image_output=%s" % args.o])

    # Look for image types
    img_types = set(args.t.split(','))
    unsupported_types = img_types.difference(set(('raw', 'qcow2')))
    if unsupported_types:
        raise RuntimeError("Unsupported type: %s" % unsupported_types)
    if "raw" in img_types:
        cmd.extend(["-e", "raw_type=True"])
    if "qcow2" in img_types:
        cmd.extend(["-e", "qcow2_type=True"])

    # Execute the playbook
    print("Running: %s" % " ".join(cmd))
    return subprocess.Popen(cmd).wait()


if __name__ == "__main__":
    exit(main())

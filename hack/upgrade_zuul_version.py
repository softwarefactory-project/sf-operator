import os
import re
import sys
import argparse
from pathlib import Path

"""
Usage:
    python hack/upgrade_zuul_version.py --rel-num 1 --zuul-version 11.2.0 \
    --hash 9f118634ca4150b850966a38194af24c943f2aae \
    --container-repo "/path/to/containers/" \
    --sf-operator-repo "/path/to/sf-operator/"

This script automates the process of updating hashes and versions
in the sf-operator and containers repositories for a new Zuul release.

The `--hash` argument is optional. If it is not provided, only the
sf-operator repository will be updated. To update both repositories,
run the script twice:
1. First without `--hash` to update the containers repository.
2. Then again with the generated hash from the first step using `--hash`.

The script is idempotent.
"""


def print_manual_steps(new_version):
    """
    Prints detailed manual steps required after running the script.

    Args:
        new_version (str): The new Zuul version to include
        in the release notes.
    """
    formatted_version = new_version.replace('.', '-')
    zuul_release_notes = (
        f"https://zuul-ci.org/docs/zuul/latest/releasenotes.html"
        f"#relnotes-{formatted_version}/"
    )
    print("\nManual Steps Required:\n")
    print("1. Update the containers repository:")
    print("   - Run the following commands:")
    print("       make update-pip-freeze")
    print("       make\n")

    print("2. Commit the changes in the containers repository:")
    print(
        "   - Take note of the commit hash "
        "(referred to as CONTAINERS_HASH).\n"
    )

    print("3. Rerun this script with the generated hash:")
    print("   - Use the `--hash CONTAINERS_HASH` option.\n")

    print("4. Update the sf-operator repository:")
    print("   - Add an entry in `CHANGELOG.md`.")
    print(
        f"   - Include a link to the Zuul release notes: "
        f"{zuul_release_notes}\n"
    )


# Updates the containers in the specified repository with the latest hashes.
# After running this function, ensure you add an entry to the CHANGELOG.md,
# including a link to the corresponding Zuul release notes for reference.
def update_sf_operator_repo(repo, new_version, new_hash):
    print(f"Processing {repo}...")

    def update_container_images(file_path):
        """Update versions and hashes in a given file."""
        common_url = (
            "https://softwarefactory-project.io/cgit/containers/"
            "tree/images-sf/master/containers/rendered"
        )
        CONTAINERS = {
            "zuul-scheduler": f"{common_url}/zuul-scheduler.container",
            "zuul-executor": f"{common_url}/zuul-executor.container",
            "zuul-merger": f"{common_url}/zuul-merger.container",
            "zuul-web": f"{common_url}/zuul-web.container",
        }

        with open(file_path, "r") as f:
            content = f.read()

        # Update version and hashes for each container
        for container_name, container_url in CONTAINERS.items():
            # container hash
            content = re.sub(
                rf"({container_url}\?id=)[a-f0-9]{{40}}",
                rf"\g<1>{new_hash}",
                content,
            )
            # version field
            content = re.sub(
                rf"(version:\s+)\d+\.\d+\.\d+-\d+"
                rf"(\n\s*{container_url}\?id={new_hash})",
                rf"\g<1>{new_version}\2",
                content,
            )

        with open(file_path, "w") as f:
            f.write(content)

    file_path = Path(repo) / "controllers/libs/base/static/images.yaml"
    if file_path.exists():
        update_container_images(file_path)
    else:
        raise FileNotFoundError(f"File {file_path} not found in {repo}.")


# Updates the specified container repository with the new Zuul
# version, release number, and hash. To complete the update process,
# you must manually run the following commands in the repository:
#
# 1. make update-pip-freeze
# 2. make
def update_containers_repo(repo, new_version, release_number):
    print(f"Processing {repo}...")

    patterns = {
        r'(\brelease =\n\s+")\d+(")': rf"\g<1>{release_number}\2",
        r'(, zuul\.master = \")\d+\.\d+\.\d+(\")': rf"\g<1>{new_version}\2",
    }

    def update_file_content(file_path):
        with open(file_path, "r") as f:
            content = f.read()

        for pattern, replacement in patterns.items():
            content = re.sub(pattern, replacement, content)

        with open(file_path, "w") as f:
            f.write(content)

    for f in ("images-sf/master/zuul.dhall",
              "images-sf/master/versions.dhall"):
        file_path = Path(repo) / f
        if file_path.exists():
            update_file_content(file_path)
        else:
            raise FileNotFoundError(f"File {file_path} not found in {repo}.")


# Main
def main():
    parser = argparse.ArgumentParser(
        description="Upgrade Zuul version and related repositories."
    )
    parser.add_argument('--zuul-version', type=str, required=True,
                        help="Zuul version to upgrade to.")
    parser.add_argument('--rel-num', type=str, required=True,
                        help="Release number.")
    parser.add_argument('--hash', type=str, default=None,
                        help="New container hash.")
    parser.add_argument('--container-repo', type=str, required=True,
                        help="Path to containers repository.")
    parser.add_argument('--sf-operator-repo', type=str, required=True,
                        help="Path to SF operator repository.")

    args = parser.parse_args()
    # Expand ~ in the paths
    args.container_repo = os.path.expanduser(args.container_repo)
    args.sf_operator_repo = os.path.expanduser(args.sf_operator_repo)

    try:
        # Update repositories
        update_containers_repo(args.container_repo,
                               args.zuul_version,
                               args.rel_num)
        if args.hash is not None:
            update_sf_operator_repo(args.sf_operator_repo,
                                    args.zuul_version,
                                    args.hash)
        else:
            print("Skipping sf-operator repo update because --hash "
                  "is not provided.")

        print_manual_steps(args.zuul_version)
    except Exception as e:
        print(f"Error: {e}")
        sys.exit(1)


# Main
if __name__ == "__main__":
    main()

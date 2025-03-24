import unittest
from io import StringIO
from unittest.mock import patch, mock_open, call
from pathlib import Path
from upgrade_zuul_version import update_sf_operator_repo
from upgrade_zuul_version import update_containers_repo


class TestScriptFunctions(unittest.TestCase):

    @patch("sys.stdout", new_callable=StringIO)
    @patch("upgrade_zuul_version.Path.exists", return_value=False)
    def test_update_sf_operator_repo_file_not_found(self, mock_e, mock_stdout):
        with self.assertRaises(FileNotFoundError):
            update_sf_operator_repo("foo", "bar", "baz")

    # Test that verifies if the correct file is read and updated in
    # update_sf_operator_repo
    @patch("sys.stdout", new_callable=StringIO)
    @patch("upgrade_zuul_version.open",
           new_callable=mock_open,
           read_data="version: 11.1.0")
    @patch("upgrade_zuul_version.Path.exists", return_value=True)
    @patch("upgrade_zuul_version.Path.__truediv__",
           return_value=(
               Path("path_to_repo/controllers/libs/base/static/images.yaml")
           ))
    def test_update_sf_operator_repo(self,
                                     mock_truediv,
                                     mock_exists,
                                     mock_file,
                                     mock_stdout):
        yaml_file = "controllers/libs/base/static/images.yaml"
        try:
            update_sf_operator_repo("path_to_repo",
                                    "1",
                                    "26135f96d13f7b5d4d0420a03581acafce2b99b8")
            mock_file.assert_has_calls(
                [
                    call(Path("path_to_repo") / yaml_file, "w"),
                    call(Path("path_to_repo") / yaml_file, "r")
                ],
                any_order=True
            )

            mock_truediv.assert_called_with(
                "controllers/libs/base/static/images.yaml"
            )

            mock_file().write.assert_called()

        except Exception as e:
            self.fail(
                f"update_sf_operator_repo() raised Exception unexpectedly: "
                f"{str(e)}"
            )

    @patch("sys.stdout", new_callable=StringIO)
    @patch("upgrade_zuul_version.Path.exists", return_value=False)
    def test_update_containers_repo_file_not_found(self, mock_e, mock_stdout):
        with self.assertRaises(FileNotFoundError):
            update_containers_repo("foo", "bar", "baz")

    # Test that verifies if the correct file is read and updated in
    # update_containers_repo
    @patch("sys.stdout", new_callable=StringIO)
    @patch("upgrade_zuul_version.open",
           new_callable=mock_open,
           read_data="release = '1.0.0'\nzuul.master = \"11.1.0\"")
    @patch("upgrade_zuul_version.Path.exists", return_value=True)
    def test_update_containers_repo(self, mock_exists, mock_file, mock_stdout):
        try:
            update_containers_repo("path_to_repo",
                                   "1",
                                   "26135f96d13f7b5d4d0420a03581acafce2b99b8")

            # Assert each file is read and write
            mock_file.assert_any_call(
                Path("path_to_repo/images-sf/master/zuul.dhall"), "r"
            )
            mock_file.assert_any_call(
                Path("path_to_repo/images-sf/master/zuul.dhall"), "w"
            )
            mock_file.assert_any_call(
                Path("path_to_repo/images-sf/master/versions.dhall"), "r"
            )
            mock_file.assert_any_call(
                Path("path_to_repo/images-sf/master/versions.dhall"), "w"
            )

        except Exception as e:
            self.fail(
                f"update_containers_repo() raised Exception unexpectedly: "
                f"{str(e)}"
            )

    @patch("sys.stdout", new_callable=StringIO)
    @patch("upgrade_zuul_version.update_sf_operator_repo")
    @patch("upgrade_zuul_version.update_containers_repo")
    def test_hash_is_none_skips_sf_operator_repo(
            self,
            mock_update_containers,
            mock_update_sf_operator,
            mock_stdout):
        try:
            # Not passing --hash
            main_args = [
                "--zuul-version", "11.2.0",
                "--rel-num", "1",
                "--container-repo", "/path/to/containers",
                "--sf-operator-repo", "/path/to/sf-operator"
            ]
            with patch("sys.argv", ["script_name"] + main_args):
                import upgrade_zuul_version
                upgrade_zuul_version.main()

            # Verify that `update_sf_operator_repo` was not called
            mock_update_sf_operator.assert_not_called()

            # Verify that `update_containers_repo` was called
            mock_update_containers.assert_called_once_with(
                "/path/to/containers", "11.2.0", "1"
            )
        except Exception as e:
            self.fail(
                f"Test for skipping sf_operator_repo when hash is None "
                f"failed: {str(e)}"
            )


if __name__ == "__main__":
    unittest.main()

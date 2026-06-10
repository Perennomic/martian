#!/usr/bin/env python3

"""Tests for the Martian Python stage shell."""

import os
import sys
import tempfile
import unittest
from unittest import mock

sys.path.insert(0, os.path.dirname(__file__))

import martian_shell  # pylint: disable=wrong-import-position


class ControlFileTest(unittest.TestCase):
    """Tests for file-backed adapter control channels."""

    def test_control_files(self):
        with tempfile.TemporaryDirectory() as tmp:
            metadata_path = os.path.join(tmp, "metadata")
            files_path = os.path.join(tmp, "files")
            os.mkdir(metadata_path)
            os.mkdir(files_path)

            log_path = os.path.join(tmp, "stage.log")
            error_path = os.path.join(tmp, "stage.errors")
            progress_path = os.path.join(metadata_path, "_progress")
            journal_prefix = os.path.join(tmp, "journal.")
            with open(error_path, "w", encoding="utf-8") as error_file:
                error_file.write("old error\n")

            env = {
                martian_shell._CONTROL_LOG_PATH_ENV: log_path,
                martian_shell._CONTROL_ERROR_PATH_ENV: error_path,
            }
            with mock.patch.dict(os.environ, env):
                metadata = martian_shell._Metadata(
                    metadata_path, files_path, journal_prefix
                )
                try:
                    metadata.log("info", "python adapter log")
                    metadata.progress("python adapter progress")
                finally:
                    metadata._logfile.close()
                martian_shell._Metadata.write_errors("python adapter error")

            with open(log_path, encoding="utf-8") as log_file:
                self.assertIn("python adapter log", log_file.read())
            with open(progress_path, "rb") as progress_file:
                self.assertEqual(
                    progress_file.read(), b"python adapter progress"
                )
            self.assertTrue(os.path.exists(journal_prefix + "progress"))
            with open(error_path, encoding="utf-8") as error_file:
                errors = error_file.read()
            self.assertEqual(errors, "python adapter error")

    def test_write_atomic_replaces_existing_file(self):
        with tempfile.TemporaryDirectory() as tmp:
            metadata_path = os.path.join(tmp, "metadata")
            files_path = os.path.join(tmp, "files")
            os.mkdir(metadata_path)
            os.mkdir(files_path)

            env = {
                martian_shell._CONTROL_LOG_PATH_ENV: os.path.join(
                    tmp, "stage.log"
                ),
            }
            with mock.patch.dict(os.environ, env):
                metadata = martian_shell._Metadata(
                    metadata_path, files_path, os.path.join(tmp, "journal.")
                )
                try:
                    metadata.write_raw(b"jobinfo", b"old")
                    metadata.write_atomic(b"jobinfo", {"new": True})
                finally:
                    metadata._logfile.close()

            with open(
                os.path.join(metadata_path, "_jobinfo"), encoding="utf-8"
            ) as jobinfo:
                self.assertIn('"new": true', jobinfo.read())


if __name__ == "__main__":
    unittest.main()

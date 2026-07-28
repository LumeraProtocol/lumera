#!/usr/bin/env python3
"""Adversarial validation for the EVM migration compatibility manifest."""

import copy
import json
import unittest
from pathlib import Path

from jsonschema import Draft202012Validator
from jsonschema.exceptions import ValidationError


REPO_ROOT = Path(__file__).resolve().parents[2]
ARTIFACT_DIR = REPO_ROOT / "docs/evm-integration/operator-artifacts"
SCHEMA_PATH = ARTIFACT_DIR / "compatibility-manifest.schema.json"
TEMPLATE_PATH = ARTIFACT_DIR / "compatibility-manifest.template.json"


def load_json(path: Path):
    with path.open(encoding="utf-8") as handle:
        return json.load(handle)


def approved_manifest(template):
    manifest = copy.deepcopy(template)
    manifest.pop("_signing_blocker", None)

    manifest["release"] = {
        "tag": "v1.20.0",
        "commit": "1" * 40,
        "release_url": "https://example.com/releases/v1.20.0",
    }
    manifest["chain"] = {
        "chain_id": "lumera-mainnet-1",
        "evm_chain_id": 1234,
        "minimum_upgrade": "v1.20.0",
    }

    for name, executable in manifest["artifacts"].items():
        if not name.endswith("_executable"):
            continue
        executable.update(
            release_path=f"/opt/lumera/bin/{executable['name']}",
            version="1.20.0",
            tag="v1.20.0",
            commit="2" * 40,
            sha256="3" * 64,
            source=f"https://example.com/releases/{executable['name']}",
        )

    manifest["artifacts"]["container_image"] = {
        "artifact": "registry.example.com/lumera/supernode:v1.20.0",
        "digest": "sha256:" + "4" * 64,
        "source": "https://registry.example.com/lumera/supernode",
    }
    for relative_name, bound_file in manifest["artifacts"]["bound_files"].items():
        bound_file.update(
            release_path=f"/opt/lumera/release/{relative_name}",
            sha256="5" * 64,
            source=f"https://example.com/releases/v1.20.0#{relative_name}",
        )

    manifest["operator_contracts"]["destination_prestage_no_echo"] = {
        "status": "verified",
        "implementation": {
            "name": "destination-prestage",
            "release_path": "/opt/lumera/bin/destination-prestage",
            "argv": [
                "/opt/lumera/bin/destination-prestage",
                "--home",
                "/var/lib/lumera",
                "--protected-fd",
                "3",
            ],
            "tag": "v1.20.0",
            "commit": "6" * 40,
            "sha256": "7" * 64,
            "source": "https://example.com/releases/destination-prestage",
            "no_echo_contract": "hidden-tty-or-protected-fd;never-argv;never-echo;no-xtrace",
        },
    }
    for check in manifest["validation"].values():
        check.update(status="pass", evidence="verified release evidence")

    manifest["approval"].update(status="approved", release_owner_approved=True)
    manifest["approval"]["detached_signature"].update(
        certificate_identity="release-owner@example.com",
        certificate_oidc_issuer="https://issuer.example.com",
    )
    manifest["_verification"] = ["All release-owner verification steps completed."]
    return manifest


class CompatibilityManifestSchemaTest(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        cls.schema = load_json(SCHEMA_PATH)
        cls.template = load_json(TEMPLATE_PATH)
        Draft202012Validator.check_schema(cls.schema)
        cls.validator = Draft202012Validator(cls.schema)
        cls.approved = approved_manifest(cls.template)

    def assert_invalid(self, manifest):
        with self.assertRaises(ValidationError):
            self.validator.validate(manifest)

    def test_blocked_template_is_schema_valid(self):
        self.validator.validate(self.template)

    def test_approved_absolute_runtime_paths_are_valid(self):
        self.validator.validate(self.approved)

    def test_unresolved_template_cannot_be_approved(self):
        unsafe = copy.deepcopy(self.template)
        unsafe["approval"]["status"] = "approved"
        self.assert_invalid(unsafe)

    def test_blocked_relative_runtime_paths_are_rejected(self):
        relative_executable = copy.deepcopy(self.template)
        relative_executable["artifacts"]["chain_executable"]["release_path"] = (
            "bin/lumerad"
        )
        self.assert_invalid(relative_executable)

        relative_file = copy.deepcopy(self.template)
        relative_file["artifacts"]["bound_files"]["scripts/migrate-validator.sh"][
            "release_path"
        ] = "scripts/migrate-validator.sh"
        self.assert_invalid(relative_file)

    def test_approved_relative_executable_release_path_is_rejected(self):
        unsafe = copy.deepcopy(self.approved)
        unsafe["artifacts"]["chain_executable"]["release_path"] = "bin/lumerad"
        self.assert_invalid(unsafe)

    def test_approved_relative_bound_file_release_path_is_rejected(self):
        unsafe = copy.deepcopy(self.approved)
        unsafe["artifacts"]["bound_files"]["scripts/migrate-validator.sh"][
            "release_path"
        ] = "scripts/migrate-validator.sh"
        self.assert_invalid(unsafe)

    def test_approved_prestage_requires_absolute_runtime_binding(self):
        missing_path = copy.deepcopy(self.approved)
        del missing_path["operator_contracts"]["destination_prestage_no_echo"][
            "implementation"
        ]["release_path"]
        self.assert_invalid(missing_path)

        relative_path = copy.deepcopy(self.approved)
        relative_path["operator_contracts"]["destination_prestage_no_echo"][
            "implementation"
        ]["release_path"] = "bin/destination-prestage"
        self.assert_invalid(relative_path)

        placeholder_path = copy.deepcopy(self.approved)
        placeholder_path["operator_contracts"]["destination_prestage_no_echo"][
            "implementation"
        ]["release_path"] = "/REPLACE_WITH_DESTINATION_PRESTAGE"
        self.assert_invalid(placeholder_path)

    def test_approved_prestage_requires_exact_non_placeholder_argv(self):
        missing_argv = copy.deepcopy(self.approved)
        del missing_argv["operator_contracts"]["destination_prestage_no_echo"][
            "implementation"
        ]["argv"]
        self.assert_invalid(missing_argv)

        relative_executable = copy.deepcopy(self.approved)
        relative_executable["operator_contracts"]["destination_prestage_no_echo"][
            "implementation"
        ]["argv"][0] = "bin/destination-prestage"
        self.assert_invalid(relative_executable)

        placeholder_argument = copy.deepcopy(self.approved)
        placeholder_argument["operator_contracts"]["destination_prestage_no_echo"][
            "implementation"
        ]["argv"][2] = "REPLACE_WITH_HOME"
        self.assert_invalid(placeholder_argument)

    def test_url_sources_are_not_constrained_as_runtime_paths(self):
        manifest = copy.deepcopy(self.approved)
        manifest["release"]["release_url"] = "https://example.com/release"
        manifest["artifacts"]["chain_executable"]["source"] = (
            "https://example.com/releases/lumerad"
        )
        manifest["artifacts"]["bound_files"]["scripts/migrate-validator.sh"][
            "source"
        ] = "https://example.com/releases/scripts#migrate-validator.sh"
        self.validator.validate(manifest)


if __name__ == "__main__":
    unittest.main()

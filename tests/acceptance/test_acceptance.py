"""Black-box acceptance tests for the Go producer/consumer take-home.

The default suite performs static checks only. Set these environment variables for
deeper checks against a completed implementation:

    RUN_COMMAND_ACCEPTANCE=1  run go/make/binary checks
    RUN_HTTP_ACCEPTANCE=1     run HTTP checks against a running stack
"""

from __future__ import annotations

import os
import re
import subprocess
import time
import unittest
import urllib.error
import urllib.request
from pathlib import Path


REPO_ROOT = Path(__file__).resolve().parents[2]


def read_text(path: Path) -> str:
    return path.read_text(encoding="utf-8")


def find_first(*patterns: str) -> Path | None:
    for pattern in patterns:
        matches = sorted(REPO_ROOT.glob(pattern))
        if matches:
            return matches[0]
    return None


def all_text_files(*patterns: str) -> list[Path]:
    files: list[Path] = []
    for pattern in patterns:
        files.extend(path for path in REPO_ROOT.glob(pattern) if path.is_file())
    return sorted(set(files))


def combined_text(*patterns: str) -> str:
    chunks: list[str] = []
    for path in all_text_files(*patterns):
        try:
            chunks.append(read_text(path))
        except UnicodeDecodeError:
            continue
    return "\n".join(chunks)


def run_command(args: list[str], timeout: int = 120) -> subprocess.CompletedProcess[str]:
    return subprocess.run(
        args,
        cwd=REPO_ROOT,
        text=True,
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        timeout=timeout,
        check=False,
    )


def require_success(test: unittest.TestCase, result: subprocess.CompletedProcess[str]) -> None:
    test.assertEqual(
        result.returncode,
        0,
        f"Command failed: {' '.join(result.args)}\n\n{result.stdout}",
    )


def http_request(
    url: str,
    *,
    method: str = "GET",
    body: bytes | None = None,
    content_type: str = "application/json",
    timeout: float = 5.0,
) -> tuple[int, str]:
    headers = {}
    if body is not None:
        headers["Content-Type"] = content_type
    request = urllib.request.Request(url, data=body, headers=headers, method=method)
    try:
        with urllib.request.urlopen(request, timeout=timeout) as response:
            return response.status, response.read().decode("utf-8", errors="replace")
    except urllib.error.HTTPError as exc:
        return exc.code, exc.read().decode("utf-8", errors="replace")


def wait_for_http(url: str, timeout_seconds: float = 30.0) -> tuple[int, str]:
    deadline = time.time() + timeout_seconds
    last_error = ""
    while time.time() < deadline:
        try:
            status, body = http_request(url, timeout=3.0)
            if status < 500:
                return status, body
            last_error = f"HTTP {status}: {body[:200]}"
        except OSError as exc:
            last_error = str(exc)
        time.sleep(1.0)
    raise AssertionError(f"Timed out waiting for {url}. Last error: {last_error}")


class StaticAcceptanceTests(unittest.TestCase):
    """Static checks against the public project contract in Plan.md."""

    def test_required_scaffold_files_exist(self) -> None:
        expected = [
            "go.mod",
            "README.md",
            "Makefile",
            "Dockerfile",
            "docker-compose.yml",
            "sqlc.yaml",
        ]
        missing = [name for name in expected if not (REPO_ROOT / name).exists()]
        self.assertFalse(missing, f"Missing required project files: {missing}")

        for path in ["cmd/producer", "cmd/consumer", "cmd/migrate"]:
            self.assertTrue((REPO_ROOT / path).is_dir(), f"Missing command package: {path}")

    def test_makefile_declares_demo_and_quality_targets(self) -> None:
        makefile = REPO_ROOT / "Makefile"
        self.assertTrue(makefile.exists(), "Missing Makefile")
        text = read_text(makefile)
        targets = [
            "build",
            "build-stripped",
            "build-pgo",
            "compare-builds",
            "coverage",
            "sqlc",
            "migrate-up",
            "migrate-down",
            "migrate-down-all",
            "migrate-status",
            "profile-cpu",
            "profile-heap",
            "profile-trace",
            "flamegraph",
            "up-infra",
            "up-producer",
            "up-consumer",
            "up-all",
        ]
        missing = [
            target
            for target in targets
            if not re.search(rf"^{re.escape(target)}\s*:", text, flags=re.MULTILINE)
        ]
        self.assertFalse(missing, f"Missing Makefile targets: {missing}")

    def test_dockerization_matches_plan(self) -> None:
        dockerfile = REPO_ROOT / "Dockerfile"
        compose = find_first("docker-compose.yml", "compose.yml", "compose.yaml")
        self.assertTrue(dockerfile.exists(), "Missing Dockerfile")
        self.assertIsNotNone(compose, "Missing Docker Compose file")

        docker_text = read_text(dockerfile)
        compose_text = read_text(compose) if compose else ""

        for name in ["producer", "consumer", "migrate"]:
            self.assertRegex(
                docker_text,
                rf"(?im)^\s*FROM\s+.+\s+AS\s+{name}\b|--target\s+{name}\b",
                f"Dockerfile should expose a {name} build target or equivalent target reference",
            )

        self.assertRegex(
            docker_text,
            r"(?i)(distroless|scratch|alpine)",
            "Dockerfile should use a minimal runtime image",
        )
        self.assertRegex(
            docker_text,
            r"(?im)^\s*USER\s+(?!root\b).+",
            "Runtime image should run as a non-root user",
        )

        for service in ["postgres", "prometheus", "grafana", "producer", "consumer"]:
            self.assertRegex(
                compose_text,
                rf"(?m)^\s{{2}}{service}\s*:",
                f"Compose file should define service {service}",
            )

        for profile in ["infra", "producer", "consumer"]:
            self.assertIn(profile, compose_text, f"Compose file should include {profile!r} profile")

    def test_database_tooling_and_migrations_exist(self) -> None:
        self.assertTrue((REPO_ROOT / "sqlc.yaml").exists(), "Missing sqlc.yaml")
        migration_text = combined_text(
            "migrations/*.sql",
            "db/migrations/*.sql",
            "internal/**/migrations/*.sql",
            "sql/migrations/*.sql",
        )
        self.assertIn("CREATE TABLE", migration_text.upper(), "Migrations should create tables")
        self.assertRegex(
            migration_text,
            r"(?is)create\s+table\s+.*tasks",
            "Migrations should create tasks table",
        )
        for column in ["id", "type", "value", "state", "creation_time", "last_update_time"]:
            self.assertRegex(
                migration_text,
                rf"(?i)\b{column}\b",
                f"Tasks migration should include column {column}",
            )
        self.assertRegex(
            migration_text,
            r"(?is)comment",
            "Migrations should include live-demo comment column migration",
        )
        self.assertRegex(
            migration_text,
            r"(?is)drop\s+column\s+.*comment|drop\s+.*comment",
            "Down migration should remove comment column",
        )

    def test_openapi_documents_consumer_task_endpoint(self) -> None:
        spec = find_first(
            "openapi*.yaml",
            "openapi*.yml",
            "api/*.yaml",
            "api/*.yml",
            "docs/openapi*.yaml",
            "docs/openapi*.yml",
        )
        self.assertIsNotNone(spec, "Missing OpenAPI/Swagger YAML spec")
        text = read_text(spec) if spec else ""
        self.assertRegex(text, r"(?im)^\s*openapi\s*:", "Spec should be OpenAPI 3.x YAML")
        self.assertIn("/tasks", text, "Spec should document POST /tasks")
        self.assertRegex(text, r"(?is)\bpost\s*:", "Spec should document POST method")
        for field in ["id", "type", "value"]:
            self.assertRegex(text, rf"(?i)\b{field}\b", f"Spec should include {field} field")
        for status in ["200", "202", "400", "404", "429"]:
            self.assertRegex(text, rf"['\"]?{status}['\"]?\s*:", f"Spec should document {status}")

    def test_readme_covers_pdf_demo_and_evaluation_topics(self) -> None:
        readme = REPO_ROOT / "README.md"
        self.assertTrue(readme.exists(), "Missing README.md")
        text = read_text(readme).lower()
        required_phrases = [
            "quickstart",
            "communication",
            "database",
            "logging",
            "file structure",
            "trade-offs",
            "scaling",
            "bottlenecks",
            "goroutine",
            "channel",
            "mutex",
            "gogc",
            "gomemlimit",
            "flamegraph",
            "pprof",
            "trace",
            "coverage",
            "migrate-up",
            "migrate-down",
            "producer -version",
            "consumer -version",
        ]
        missing = [phrase for phrase in required_phrases if phrase not in text]
        self.assertFalse(missing, f"README is missing required demo topics: {missing}")

    def test_source_mentions_required_runtime_features(self) -> None:
        source = combined_text("cmd/**/*.go", "internal/**/*.go", "pkg/**/*.go")
        required_patterns = {
            "slog logging": r"\blog/slog\b|slog\.",
            "signal shutdown": r"signal\.NotifyContext|os\.Signal",
            "pprof profiling": r"net/http/pprof|/debug/pprof",
            "embed usage": r"//go:embed|embed\.",
            "prometheus metrics": r"prometheus\.|promhttp",
            "rate limiting": r"rate\.Limiter|rate limit|RateLimiter|ticker",
            "stale processing reaper": r"reaper|stale.*processing|processing.*stale",
        }
        for label, pattern in required_patterns.items():
            self.assertRegex(source, pattern, f"Source should include {label}")


@unittest.skipUnless(
    os.environ.get("RUN_COMMAND_ACCEPTANCE") == "1",
    "set RUN_COMMAND_ACCEPTANCE=1 to run command checks",
)
class CommandAcceptanceTests(unittest.TestCase):
    def test_go_tests_pass(self) -> None:
        self.assertTrue((REPO_ROOT / "go.mod").exists(), "Missing go.mod")
        result = run_command(["go", "test", "./..."], timeout=180)
        require_success(self, result)

    def test_make_build_passes(self) -> None:
        self.assertTrue((REPO_ROOT / "Makefile").exists(), "Missing Makefile")
        result = run_command(["make", "build"], timeout=180)
        require_success(self, result)

    def test_built_binaries_report_version(self) -> None:
        candidates = [
            REPO_ROOT / "bin" / "producer",
            REPO_ROOT / "bin" / "consumer",
        ]
        missing = [str(path.relative_to(REPO_ROOT)) for path in candidates if not path.exists()]
        self.assertFalse(missing, f"Missing built binaries after make build: {missing}")

        for binary in candidates:
            result = run_command([str(binary), "-version"], timeout=10)
            require_success(self, result)
            self.assertRegex(
                result.stdout.strip(),
                r"\S+",
                f"{binary.name} -version should print a non-empty version",
            )


@unittest.skipUnless(
    os.environ.get("RUN_HTTP_ACCEPTANCE") == "1",
    "set RUN_HTTP_ACCEPTANCE=1 to run HTTP checks against a running stack",
)
class HttpAcceptanceTests(unittest.TestCase):
    def test_observability_endpoints_are_reachable(self) -> None:
        endpoints = {
            "producer metrics": os.environ.get(
                "PRODUCER_METRICS_URL", "http://localhost:9101/metrics"
            ),
            "consumer metrics": os.environ.get(
                "CONSUMER_METRICS_URL", "http://localhost:9102/metrics"
            ),
            "prometheus ready": os.environ.get(
                "PROMETHEUS_READY_URL", "http://localhost:9090/-/ready"
            ),
            "grafana health": os.environ.get(
                "GRAFANA_HEALTH_URL", "http://localhost:3000/api/health"
            ),
        }
        for label, url in endpoints.items():
            status, body = wait_for_http(url)
            self.assertLess(status, 500, f"{label} returned HTTP {status}: {body[:200]}")

    def test_metrics_expose_required_series(self) -> None:
        producer_url = os.environ.get("PRODUCER_METRICS_URL", "http://localhost:9101/metrics")
        consumer_url = os.environ.get("CONSUMER_METRICS_URL", "http://localhost:9102/metrics")
        _, producer_metrics = wait_for_http(producer_url)
        _, consumer_metrics = wait_for_http(consumer_url)

        for metric in [
            "producer_generated_total",
            "producer_send_attempts_total",
            "producer_backlog_pause_total",
            "tasks_in_state",
        ]:
            self.assertIn(metric, producer_metrics, f"Producer metrics missing {metric}")

        for metric in [
            "consumer",
            "task",
            "type",
        ]:
            self.assertIn(metric, consumer_metrics.lower(), f"Consumer metrics missing {metric}")

    def test_swagger_yaml_is_served(self) -> None:
        swagger_url = os.environ.get("SWAGGER_URL", "http://localhost:8080/swagger.yaml")
        status, body = wait_for_http(swagger_url)
        self.assertEqual(status, 200, body[:200])
        self.assertIn("openapi", body.lower())
        self.assertIn("/tasks", body)

    def test_consumer_rejects_invalid_and_unknown_tasks(self) -> None:
        tasks_url = os.environ.get("CONSUMER_TASKS_URL", "http://localhost:8080/tasks")

        invalid_status, _ = http_request(tasks_url, method="POST", body=b"{not-json")
        self.assertEqual(invalid_status, 400, "Invalid JSON should return 400")

        unknown_status, _ = http_request(
            tasks_url,
            method="POST",
            body=b'{"id":999999999,"type":1,"value":1}',
        )
        self.assertEqual(unknown_status, 404, "Unknown task ID should return 404")


if __name__ == "__main__":
    unittest.main(verbosity=2)

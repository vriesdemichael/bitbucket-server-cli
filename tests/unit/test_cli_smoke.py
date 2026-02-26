from typer.testing import CliRunner

from bitbucket_server_cli.cli import app


def test_auth_status_smoke() -> None:
    runner = CliRunner()
    result = runner.invoke(app, ["auth", "status"])
    assert result.exit_code == 0
    assert "Target Bitbucket" in result.stdout

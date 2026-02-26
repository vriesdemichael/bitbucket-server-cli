import typer

from bitbucket_server_cli.config import AppConfig

app = typer.Typer(help="Bitbucket Server CLI (live-behavior first)", no_args_is_help=True)
auth_app = typer.Typer(help="Authentication commands")
repo_app = typer.Typer(help="Repository commands")
pr_app = typer.Typer(help="Pull request commands")
issue_app = typer.Typer(help="Issue commands")
admin_app = typer.Typer(help="Local environment/admin commands")

app.add_typer(auth_app, name="auth")
app.add_typer(repo_app, name="repo")
app.add_typer(pr_app, name="pr")
app.add_typer(issue_app, name="issue")
app.add_typer(admin_app, name="admin")


@auth_app.command("status")
def auth_status() -> None:
    config = AppConfig()
    typer.echo(
        f"Target Bitbucket: {config.bitbucket_url} (expected version {config.bitbucket_version_target})"
    )


@repo_app.command("list")
def repo_list() -> None:
    typer.echo("TODO: implement live repo listing")


@pr_app.command("list")
def pr_list() -> None:
    typer.echo("TODO: implement live PR listing")


@issue_app.command("list")
def issue_list() -> None:
    typer.echo("TODO: implement live issue listing")


@admin_app.command("health")
def admin_health() -> None:
    typer.echo("TODO: implement local stack health check")


if __name__ == "__main__":
    app()

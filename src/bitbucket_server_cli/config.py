from pydantic import BaseModel, Field


class AppConfig(BaseModel):
    bitbucket_url: str = Field(default="http://localhost:7990")
    bitbucket_version_target: str = Field(default="9.4.16")
    project_key: str = Field(default="TEST")

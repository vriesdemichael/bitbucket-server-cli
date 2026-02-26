from dataclasses import dataclass


@dataclass
class BitbucketClient:
    base_url: str
    token: str | None = None

    def get(self, path: str) -> None:
        raise NotImplementedError("Implement HTTP transport with retries/timeouts")

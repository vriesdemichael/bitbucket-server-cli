from enum import StrEnum
from typing import Any, Generic, Literal, Self, TypeVar

from pydantic import BaseModel, ConfigDict


class Project(BaseModel):
    key: str
    id: int
    name: str
    description: str | None = None
    public: bool
    type: str
    links: dict[str, list[dict[str, str]]]

    def __str__(self):
        return f"Project key={self.key}"


class Repository(BaseModel):
    slug: str
    id: int
    name: str
    hierarchyId: str
    scmId: str
    state: str
    statusMessage: str
    forkable: bool
    project: Project
    public: bool
    archived: bool
    links: dict[str, list[dict[str, str]]]

    def __str__(self):
        return f"<Repo: {self.project.key}/{self.slug}>"


class User(BaseModel):
    id: int
    name: str
    active: bool
    displayName: str
    slug: str
    type: str
    emailAddress: str | None = None
    links: dict[str, list[dict[str, str]]] | None = None


class RepoCondition(BaseModel):
    id: int
    scope: dict[str, Any]
    sourceRefMatcher: Any
    targetRefMatcher: Any
    reviewers: list[User]
    requiredApprovals: int


class RefType(StrEnum):
    BRANCH = "BRANCH"
    TAG = "TAG"


class Ref(BaseModel):
    id: str
    type: RefType
    displayId: str
    latestCommit: str
    repository: Repository


class ParticipantStatus(StrEnum):
    UNAPPROVED = "UNAPPROVED"
    NEEDS_WORK = "NEEDS_WORK"
    APPROVED = "APPROVED"


class ParticipantRole(StrEnum):
    AUTHOR = "AUTHOR"
    REVIEWR = "REVIEWER"
    PARTICIPANT = "PARTICIPANT"


class Participant(BaseModel):
    status: ParticipantStatus
    user: User
    role: ParticipantRole
    lastReviewedCommit: str | None = None
    approved: bool


class PullRequestState(StrEnum):
    DECLINED = "DECLINED"
    MERGED = "MERGED"
    OPEN = "OPEN"


class RestPullRequest(BaseModel):
    id: int
    state: PullRequestState
    open: bool
    locked: bool
    version: int
    createdDate: int
    description: str | None = None
    toRef: Ref
    title: str
    closed: bool
    closedDate: int | None = None
    fromRef: Ref
    participants: list[Participant]
    reviewers: list[Participant]
    updatedDate: int


T = TypeVar("T")


class RestFileReference(BaseModel):
    # file extension only
    extension: str | None = None
    # filename including extension
    name: str | None = None
    # Path to parent
    parent: str | None = None
    # path split into parts
    components: list[str] | None = None


class PagedResult(BaseModel, Generic[T]):
    """Most REST calls that return paged results are converted into a
    generator by the atlassian module, this is for the calls in which they forget to implement it"""

    values: list[T]
    size: int
    limit: int
    isLastPage: bool
    nextPageStart: int | None = None
    start: int


class RestWebhookCredentials(BaseModel):
    username: str
    password: str


class WebhookEvent(StrEnum):
    """gathered by enabling all options in a webhook and retrieving the webhook :)"""

    REPO_REFS_CHANGED = "repo:refs_changed"
    REPO_FORKED = "repo:forked"
    REPO_COMMENT_ADDED = "repo:comment:added"
    REPO_COMMENT_EDITED = "repo:comment:edited"
    REPO_COMMENT_DELETED = "repo:comment:deleted"
    REPO_SECRET_DETECTED = "repo:secret_detected"
    PR_OPENED = "pr:opened"
    PR_MODIFIED = "pr:modified"
    PR_MERGED = "pr:merged"
    PR_DECLINED = "pr:declined"
    PR_DELETED = "pr:deleted"
    PR_FROM_REF_UPDATED = "pr:from_ref_updated"
    PR_COMMENT_DELETED = "pr:comment:deleted"
    PR_COMMENT_ADDED = "pr:comment:added"
    PR_COMMENT_EDITED = "pr:comment:edited"
    PR_REVIEWER_UPDATED = "pr:reviewer:updated"
    PR_REVIEWER_NEEDS_WORK = "pr:reviewer:needs_work"
    PR_REVIEWER_APPROVED = "pr:reviewer:approved"
    PR_REVIEWER_UNAPPROVED = "pr:reviewer:unapproved"
    PR_REVIEWER_MODIFIED = "repo:modified"
    MIRROR_REPO_SYNCHRONIZED = "mirror:repo_synchronized"


class RestWebhook(BaseModel):
    id: int
    name: str
    createdDate: int
    updatedDate: int
    events: list[WebhookEvent]
    configuration: dict[Any, Any]  # No further info available
    active: bool
    url: str
    scopeType: str = "repository"  # always repository? perhaps project
    sslVerificationRequired: bool
    credentials: RestWebhookCredentials | None = None

    def __str__(self):
        return f"Webhook(name={self.name}, url={self.url})"


class RestMinimalRef(BaseModel):
    id: str
    type: RefType
    displayId: str


class RestLabel(BaseModel):
    name: str


class RestCommentAnchor(BaseModel):
    path: RestFileReference | None = None
    diffType: Literal["COMMIT", "EFFECTIVE", "RANGE"] | None = None
    fileType: Literal["FROM", "TO"] | None = None
    fromHash: str | None = None
    lineType: Literal["ADDED", "CONTEXT", "REMOVED"] | None = None
    pullRequest: RestPullRequest | None = None
    lineComment: bool | None = None
    line: int | None = None
    srcPath: RestFileReference | None = None
    toHash: str | None = None


class RestComment(BaseModel):
    version: int
    parent: Self | None = None
    id: int | None = None
    state: str | None = None
    threadResolvedDate: int | None = None
    threadResolver: User | None = None
    threadResolved: bool | None = None
    createdDate: int | None = None
    ResolvedDate: int | None = None
    Resolver: User | None = None
    updatedDate: int | None = None
    comments: list[Self] | None = None
    text: str | None = None
    anchor: RestCommentAnchor | None = None
    author: User | None = None
    html: str | None = None
    anchored: bool | None = None
    pending: bool | None = None
    reply: bool | None = None
    properties: Any = None


class PullRequestActivity(BaseModel):
    model_config = ConfigDict(extra="allow")
    action: Literal[
        "APPROVED",
        "AUTO_MERGE_CANCELLED",
        "AUTO_MERGE_REQUESTED",
        "COMMENTED",
        "DECLINED",
        "DELETED",
        "MERGED",
        "OPENED",
        "REOPENED",
        "RESCOPED",
        "REVIEW_COMMENTED",
        "REVIEW_DISCARDED",
        "REVIEW_FINISHED",
        "REVIEWED",
        "UNAPPROVED",
        "UPDATED",
    ]
    user: User
    id: int
    createdDate: int

    # These only occur when action == Commented
    commentAction: Literal["ADDED", "DELETED", "EDITED", "REPLIED"] | None = None
    comment: RestComment | None = None
    # Note to bitbucket server devs, wtf were you smoking when designing this endpoint?

    # There are probably other (undocumented) fields in the activities


class RestDiffLine(BaseModel):
    truncated: bool | None = None
    conflictMarker: Literal["MARKER"] | Literal["OURS"] | Literal["THEIRS"] | None = (
        None
    )
    commentIds: list[int] | None = None
    destination: int | None = None
    source: int | None = None
    line: str | None = None


class RestDiffSegment(BaseModel):
    type: str | None = None
    truncated: bool | None = None
    lines: list[RestDiffLine] | None = None


class RestDiffHunk(BaseModel):
    context: str | None = None
    sourceLine: int | None = None
    segments: list[RestDiffSegment] | None = None
    sourceSpan: int | None = None
    destinationSpan: int | None = None
    destionationLine: int | None = None
    truncated: bool | None = None


class RestDiff(BaseModel):
    truncated: bool | None = None
    lineComments: list[RestComment] | None = None
    destination: RestFileReference | None = None
    source: RestFileReference | None = None
    binary: bool | None = None
    hunks: list[RestDiffHunk] | None = None
    properties: Any | None = None


class RestCommitter(BaseModel):
    name: str | None = None
    emailAddress: str | None = None


class RestMinimalCommit(BaseModel):
    id: str | None = None
    displayId: str | None = None


class RestCommit(BaseModel):
    message: str | None = None
    committerTimestamp: int | None = None
    committer: RestCommitter | None = None
    authorTimestamp: int | None = None
    parents: list[RestMinimalCommit] | None = None
    author: RestCommitter | None = None
    id: str | None = None
    displayId: str | None = None


class MinimalRestChange(BaseModel):
    type: Literal["ADD", "COPY", "DELETE", "MODIFY", "MOVE", "UNKNOWN"] | None = None
    path: RestFileReference | None = None
    srcPath: RestFileReference | None = None


class RestConflict(BaseModel):
    ourChange: MinimalRestChange
    theirChange: MinimalRestChange


class RestChange(BaseModel):
    type: Literal["ADD", "COPY", "DELETE", "MODIFY", "MOVE", "UNKNOWN"] | None = None
    path: RestFileReference | None = None
    srcExecutable: bool | None = None
    percentUnchanged: int | None = None
    conflict: RestConflict | None = None
    contentId: str | None = None
    fromContentId: str | None = None
    nodeType: Literal["DIRECTORY", "FILE", "SUBMODULE"] | None = None
    executable: bool | None = None
    srcPath: RestFileReference | None = None

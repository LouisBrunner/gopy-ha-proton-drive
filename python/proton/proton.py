import json
import pathlib
import subprocess
from dataclasses import dataclass
from typing import Callable, List, Optional

_go_exec = pathlib.Path(__file__).parent.resolve() / "_go_exec"


@dataclass
class Credentials:
    UID: str
    AccessToken: str
    RefreshToken: str
    SaltedKeyPass: str


@dataclass
class Share:
    Name: str
    ShareID: str


@dataclass
class _Result:
    Creds: Optional[Credentials]
    LinkID: Optional[str]
    DownloadedPath: Optional[str]
    Shares: Optional[List[Share]]
    Metadata: Optional[List[str]]


OnAuthChange = Callable[[Credentials], None]


def _call_go_exec(
    *commands: str,
    creds: Optional[Credentials] = None,
    link_id: str = "",
    instance_id: str = "",
    backup_id: str = "",
    name: str = "",
    metadata_json: str = "",
    content_path: str = "",
    root_folder: str = "",
    share_id: str = "",
    username: str = "",
    password: str = "",
    mfa: str = "",
    on_auth_change: Optional[OnAuthChange] = None,
) -> _Result:
    args = [_go_exec, *commands]
    if creds is not None:
        args.extend(
            [
                "--uid",
                creds.UID,
                "--access-token",
                creds.AccessToken,
                "--refresh-token",
                creds.RefreshToken,
                "--salted-key-pass",
                creds.SaltedKeyPass,
            ]
        )
    if link_id:
        args.extend(["--link-id", link_id])
    if instance_id:
        args.extend(["--instance-id", instance_id])
    if backup_id:
        args.extend(["--backup-id", backup_id])
    if name:
        args.extend(["--name", name])
    if metadata_json:
        args.extend(["--metadata-json", metadata_json])
    if content_path:
        args.extend(["--content-path", content_path])
    if root_folder:
        args.extend(["--root-folder", root_folder])
    if share_id:
        args.extend(["--share-id", share_id])
    if username:
        args.extend(["--username", username])
    if password:
        args.extend(["--password", password])
    if mfa:
        args.extend(["--mfa", mfa])
    try:
        output = subprocess.check_output(*args)
        output_dict = json.loads(output.decode("utf-8"))
        if "error" in output_dict:
            raise RuntimeError(output_dict["error"])
        creds = None
        if "creds" in output_dict:
            creds = Credentials(
                UID=output_dict["creds"]["uid"],
                AccessToken=output_dict["creds"]["access_token"],
                RefreshToken=output_dict["creds"]["refresh_token"],
                SaltedKeyPass=output_dict["creds"]["salted_key_pass"],
            )
            if on_auth_change is not None:
                on_auth_change(creds)
        shares = None
        if "shares" in output_dict:
            shares = [
                Share(ShareID=share["ShareID"], Name=share["Name"])
                for share in output_dict.get("shares")
            ]
        return _Result(
            Creds=creds,
            LinkID=output_dict.get("link_id"),
            DownloadedPath=output_dict.get("downloaded_path"),
            Shares=shares,
            Metadata=output_dict.get("metadata"),
        )
    except FileNotFoundError as error:
        raise RuntimeError(f"internal error (no Go exec): {error}") from error
    except subprocess.CalledProcessError as error:
        raise RuntimeError(f"internal error (Go exec failed): {error}") from error


class Folder:
    _client: "Client"
    _root_folder: str

    def __init__(self, client: "Client", root_folder: str):
        self._client = client
        self._root_folder = root_folder

    def FindBackup(self, instanceID: str, backupID: str) -> str:
        res = _call_go_exec(
            "with-creds",
            "find-backup",
            creds=self._client._creds,
            on_auth_change=self._client._on_auth_change,
            share_id=self._client._share_id,
            root_folder=self._root_folder,
            instance_id=instanceID,
            backup_id=backupID,
        )
        if res.LinkID is None:
            raise RuntimeError("internal error: wrong result in FindBackup")
        return res.LinkID

    def Upload(
        self,
        instanceID: str,
        backupID: str,
        name: str,
        metadataJSON: str,
        contentPath: str,
    ) -> None:
        _call_go_exec(
            "with-creds",
            "upload",
            creds=self._client._creds,
            on_auth_change=self._client._on_auth_change,
            share_id=self._client._share_id,
            root_folder=self._root_folder,
            instance_id=instanceID,
            backup_id=backupID,
            name=name,
            metadata_json=metadataJSON,
            content_path=contentPath,
        )

    def ListFilesMetadata(self, instanceID: str) -> List[str]:
        res = _call_go_exec(
            "with-creds",
            "list-files-metadata",
            creds=self._client._creds,
            on_auth_change=self._client._on_auth_change,
            share_id=self._client._share_id,
            root_folder=self._root_folder,
            instance_id=instanceID,
        )
        if res.Metadata is None:
            raise RuntimeError("internal error: wrong result in ListFilesMetadata")
        return res.Metadata


class Client:
    _creds: Credentials
    _on_auth_change: OnAuthChange
    _share_id: str

    def __init__(self, creds: Credentials, on_auth_change: OnAuthChange):
        self._creds = creds
        self._on_auth_change = on_auth_change
        _call_go_exec(
            "with-creds",
            "check",
            creds=self._creds,
            on_auth_change=self._on_auth_change,
            share_id=self._share_id,
        )

    def DownloadFile(self, linkID: str) -> str:
        res = _call_go_exec(
            "with-creds",
            "download",
            creds=self._creds,
            on_auth_change=self._on_auth_change,
            share_id=self._share_id,
            link_id=linkID,
        )
        if res.DownloadedPath is None:
            raise RuntimeError("internal error: wrong result in DownloadFile")
        return res.DownloadedPath

    def DeleteFile(self, linkID: str) -> None:
        _call_go_exec(
            "with-creds",
            "delete",
            creds=self._creds,
            on_auth_change=self._on_auth_change,
            share_id=self._share_id,
            link_id=linkID,
        )

    def MakeRootFolder(self, path: str) -> Folder:
        return Folder(client=self, root_folder=path)

    def ListShares(self) -> List[Share]:
        res = _call_go_exec(
            "with-creds",
            "list-shares",
            creds=self._creds,
            on_auth_change=self._on_auth_change,
            share_id=self._share_id,
        )
        if res.Shares is None:
            raise RuntimeError("internal error: wrong result in ListShares")
        return res.Shares

    def SelectShare(self, shareID: str) -> None:
        self._share_id = shareID
        _call_go_exec(
            "with-creds",
            "check",
            creds=self._creds,
            on_auth_change=self._on_auth_change,
            share_id=self._share_id,
        )


def NewClient(creds: Credentials, onAuthChange: OnAuthChange) -> Client:
    return Client(creds=creds, on_auth_change=onAuthChange)


def Login(username: str, password: str, mfa: str) -> Credentials:
    res = _call_go_exec(
        "with-creds",
        "login",
        username=username,
        password=password,
        mfa=mfa,
    )
    if res.Creds is None:
        raise RuntimeError("internal error: wrong result in Login")
    return res.Creds

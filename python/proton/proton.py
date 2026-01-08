import json
import logging
import os
import pathlib
import subprocess
from dataclasses import dataclass
from typing import Callable, List, Optional

_go_exec = pathlib.Path(__file__).parent.resolve() / "_go_exec"

logger: logging.Logger = logging.getLogger(__name__)
if "PROTON_LOGLEVEL" in os.environ:
    logging.basicConfig()
    logger.setLevel(os.environ["PROTON_LOGLEVEL"].upper())


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


def _redact_string(s: str, to_redact: List[str]) -> str:
    out = s
    for item in to_redact:
        if item != "":
            out = out.replace(item, "<REDACTED>")
    return out


def _get_redacted_args(
    args: List[str],
    *,
    creds: Optional[Credentials],
    username: str,
    password: str,
    mfa: str,
) -> str:
    debug_args = " ".join(args)
    return _redact_string(
        debug_args,
        [username, password, mfa]
        + (
            [creds.UID, creds.AccessToken, creds.RefreshToken, creds.SaltedKeyPass]
            if creds is not None
            else []
        ),
    )


def _get_redacted_output(output: str, output_dict: dict) -> str:
    redacted_output = output
    if (creds_data := output_dict.get("creds")) is not None:
        redacted_output = _redact_string(
            redacted_output,
            [
                creds_data["UID"],
                creds_data["AccessToken"],
                creds_data["RefreshToken"],
                creds_data["SaltedKeyPass"],
            ],
        )
    return redacted_output


def _get_redacted_res(res: _Result) -> str:
    redacted_res = _Result(
        Creds=None,
        LinkID=res.LinkID,
        DownloadedPath=res.DownloadedPath,
        Shares=res.Shares,
        Metadata=res.Metadata,
    )
    if res.Creds is not None:
        redacted_res.Creds = Credentials(
            UID="<REDACTED>",
            AccessToken="<REDACTED>",
            RefreshToken="<REDACTED>",
            SaltedKeyPass="<REDACTED>",
        )
    return str(redacted_res)


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
    args = [str(_go_exec), *commands]
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
    if link_id != "":
        args.extend(["--link-id", link_id])
    if instance_id != "":
        args.extend(["--instance-id", instance_id])
    if backup_id != "":
        args.extend(["--backup-id", backup_id])
    if name != "":
        args.extend(["--name", name])
    if metadata_json != "":
        args.extend(["--metadata-json", metadata_json])
    if content_path != "":
        args.extend(["--content-path", content_path])
    if root_folder != "":
        args.extend(["--root-folder", root_folder])
    if share_id != "":
        args.extend(["--share-id", share_id])
    if username != "":
        args.extend(["--email", username])
    if password != "":
        args.extend(["--password", password])
    if mfa != "":
        args.extend(["--mfa", mfa])
    try:
        logger.debug(
            f"Executing {_go_exec} {_get_redacted_args(args[1:], creds=creds, username=username, password=password, mfa=mfa)}"
        )

        res = subprocess.run(
            args, check=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE
        )
        output = res.stdout.decode("utf-8").strip()
        log_lines = res.stderr.decode("utf-8").splitlines()
        for line in log_lines:
            logger.debug(line)

        output_dict = json.loads(output)
        logger.debug(f"Output: {_get_redacted_output(output, output_dict)}")

        if (error := output_dict.get("error")) is not None:
            raise RuntimeError(error)
        creds = None
        if (creds_data := output_dict.get("creds")) is not None:
            creds = Credentials(
                UID=creds_data["UID"],
                AccessToken=creds_data["AccessToken"],
                RefreshToken=creds_data["RefreshToken"],
                SaltedKeyPass=creds_data["SaltedKeyPass"],
            )
            if on_auth_change is not None:
                on_auth_change(creds)
        shares = None
        if (shares_data := output_dict.get("shares")) is not None:
            shares = [
                Share(ShareID=share["ShareID"], Name=share["Name"])
                for share in shares_data
            ]
        res = _Result(
            Creds=creds,
            LinkID=output_dict.get("link_id"),
            DownloadedPath=output_dict.get("downloaded_path"),
            Shares=shares,
            Metadata=output_dict.get("metadata"),
        )
        logger.debug(f"Result: {_get_redacted_res(res)}")
        return res
    except json.JSONDecodeError as error:
        raise RuntimeError(
            f"internal error (invalid Go exec output): {error}"
        ) from error
    except UnicodeDecodeError as error:
        raise RuntimeError(
            f"internal error (invalid Go exec output encoding): {error}"
        ) from error
    except FileNotFoundError as error:
        raise RuntimeError(f"internal error (no Go exec): {error}") from error
    except subprocess.SubprocessError as error:
        raise RuntimeError(f"internal error (Go exec failed): {error}") from error


def ConfigureLogger(new_logger: logging.Logger):
    global logger
    logger = new_logger


class Folder:
    _client: "Client"
    _root_folder: str

    def __init__(self, client: "Client", root_folder: str):
        self._client = client
        self._root_folder = root_folder

    def FindBackup(self, instanceID: str, backupID: str) -> str:
        res = self._client._exec(
            "find",
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
        self._client._exec(
            "upload",
            root_folder=self._root_folder,
            instance_id=instanceID,
            backup_id=backupID,
            name=name,
            metadata_json=metadataJSON,
            content_path=contentPath,
        )

    def ListFilesMetadata(self, instanceID: str) -> List[str]:
        res = self._client._exec(
            "list-metadata",
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
        self._share_id = ""
        self._exec("check")

    def DownloadFile(self, linkID: str) -> str:
        res = self._exec(
            "download",
            link_id=linkID,
        )
        if res.DownloadedPath is None:
            raise RuntimeError("internal error: wrong result in DownloadFile")
        return res.DownloadedPath

    def DeleteFile(self, linkID: str) -> None:
        self._exec(
            "delete",
            link_id=linkID,
        )

    def MakeRootFolder(self, path: str) -> Folder:
        return Folder(client=self, root_folder=path)

    def ListShares(self) -> List[Share]:
        res = self._exec("list-shares")
        if res.Shares is None:
            raise RuntimeError("internal error: wrong result in ListShares")
        return res.Shares

    def SelectShare(self, shareID: str) -> None:
        self._share_id = shareID
        self._exec("check")

    def _handle_auth_change(self, new_creds: Credentials) -> None:
        self._creds = new_creds
        if self._on_auth_change is not None:
            self._on_auth_change(new_creds)

    def _exec(self, command: str, **kwargs) -> _Result:
        return _call_go_exec(
            "with-creds",
            command,
            creds=self._creds,
            on_auth_change=self._handle_auth_change,
            share_id=self._share_id,
            **kwargs,
        )


def NewClient(creds: Credentials, on_auth_change: OnAuthChange) -> Client:
    return Client(creds=creds, on_auth_change=on_auth_change)


def Login(username: str, password: str, mfa: str) -> Credentials:
    res = _call_go_exec(
        "login",
        username=username,
        password=password,
        mfa=mfa,
    )
    if res.Creds is None:
        raise RuntimeError("internal error: wrong result in Login")
    return res.Creds

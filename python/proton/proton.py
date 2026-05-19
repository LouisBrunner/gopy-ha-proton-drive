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
    uid: str
    access_token: str
    refresh_token: str
    salted_key_pass: str


@dataclass
class Share:
    name: str
    share_id: str


@dataclass
class _Result:
    creds: Optional[Credentials]
    downloaded_path: Optional[str]
    shares: Optional[List[Share]]
    metadata: Optional[List[str]]


OnAuthChange = Callable[[Credentials], None]


def _redact_string(s: str, to_redact: List[str | None]) -> str:
    out = s
    for item in to_redact:
        if item != "" and item is not None:
            out = out.replace(item, "<REDACTED>")
    return out


def _get_redacted_args(
    args: List[str],
    *,
    creds: Optional[Credentials],
    username: str | None,
    password: str | None,
    mailbox_password: str | None,
    mfa: str | None,
    captcha_token: str | None,
) -> str:
    debug_args = " ".join(args)
    return _redact_string(
        debug_args,
        [username, password, mailbox_password, mfa, captcha_token]
        + (
            [creds.uid, creds.access_token, creds.refresh_token, creds.salted_key_pass]
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
                creds_data["uid"],
                creds_data["access_token"],
                creds_data["refresh_token"],
                creds_data["salted_key_pass"],
            ],
        )
    return redacted_output


def _get_redacted_res(res: _Result) -> str:
    redacted_res = _Result(
        creds=None,
        downloaded_path=res.downloaded_path,
        shares=res.shares,
        metadata=res.metadata,
    )
    if res.creds is not None:
        redacted_res.creds = Credentials(
            uid="<REDACTED>",
            access_token="<REDACTED>",
            refresh_token="<REDACTED>",
            salted_key_pass="<REDACTED>",
        )
    return str(redacted_res)


def _call_go_exec(
    *commands: str,
    creds: Optional[Credentials] = None,
    instance_id: str | None = None,
    backup_id: str | None = None,
    name: str | None = None,
    metadata_json: str | None = None,
    content_path: str | None = None,
    root_folder: str | None = None,
    share_id: str | None = None,
    username: str | None = None,
    password: str | None = None,
    mailbox_password: str | None = None,
    mfa: str | None = None,
    captcha_token: str | None = None,
    log_level: str | None = None,
    upload_retries: int | None = None,
    upload_chunk_size_bytes: int | None = None,
    on_auth_change: Optional[OnAuthChange] = None,
) -> _Result:
    args = [str(_go_exec), *commands]
    if creds is not None:
        args.extend(
            [
                "--uid",
                creds.uid,
                "--access-token",
                creds.access_token,
                "--refresh-token",
                creds.refresh_token,
                "--salted-key-pass",
                creds.salted_key_pass,
            ]
        )
    if instance_id is not None:
        args.extend(["--instance-id", instance_id])
    if backup_id is not None:
        args.extend(["--backup-id", backup_id])
    if name is not None:
        args.extend(["--name", name])
    if metadata_json is not None:
        args.extend(["--metadata-json", metadata_json])
    if content_path is not None:
        args.extend(["--content-path", content_path])
    if root_folder is not None:
        args.extend(["--root-folder", root_folder])
    if share_id is not None:
        args.extend(["--share-id", share_id])
    if username is not None:
        args.extend(["--email", username])
    if password is not None:
        args.extend(["--password", password])
    if mailbox_password is not None:
        args.extend(["--mailbox-password", mailbox_password])
    if captcha_token is not None:
        args.extend(["--captcha-token", captcha_token])
    if mfa is not None:
        args.extend(["--mfa", mfa])
    if log_level is not None:
        args.extend(["--log-level", log_level])
    if upload_retries is not None:
        args.extend(["--max-tries", str(upload_retries)])
    if upload_chunk_size_bytes is not None:
        args.extend(["--chunk-size", str(upload_chunk_size_bytes)])
    try:
        redacted_args = _get_redacted_args(
            args[1:],
            creds=creds,
            username=username,
            password=password,
            mailbox_password=mailbox_password,
            mfa=mfa,
            captcha_token=captcha_token,
        )
        logger.debug(f"Executing {_go_exec} {redacted_args}")

        res = subprocess.run(
            args, check=True, stdout=subprocess.PIPE, stderr=subprocess.PIPE
        )
        output = res.stdout.decode("utf-8").strip()
        log_lines = res.stderr.decode("utf-8").splitlines()
        for line in log_lines:
            logger.info(line)

        output_dict = json.loads(output)
        logger.debug(f"Output: {_get_redacted_output(output, output_dict)}")

        if (error := output_dict.get("error")) is not None:
            if isinstance(error, dict):
                code = error.get("code", "unknown")
                message = error.get("message", "unknown")
            else:
                code = "unknown"
                message = str(error)
            raise RuntimeError(f"[{code}]: {message}")
        creds = None
        if (creds_data := output_dict.get("creds")) is not None:
            creds = Credentials(
                uid=creds_data["uid"],
                access_token=creds_data["access_token"],
                refresh_token=creds_data["refresh_token"],
                salted_key_pass=creds_data["salted_key_pass"],
            )
            if on_auth_change is not None:
                on_auth_change(creds)
        shares = None
        if (shares_data := output_dict.get("shares")) is not None:
            shares = [
                Share(share_id=share["share_id"], name=share["name"])
                for share in shares_data
            ]

        res = _Result(
            creds=creds,
            downloaded_path=output_dict.get("downloaded_path"),
            shares=shares,
            metadata=output_dict.get("metadata"),
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


def configure_logger(new_logger: logging.Logger):
    global logger
    logger = new_logger


class Folder:
    _client: "Client"
    _root_folder: str

    def __init__(self, client: "Client", root_folder: str):
        self._client = client
        self._root_folder = root_folder

    def download(
        self,
        instance_id: str,
        backup_id: str,
    ) -> str:
        res = self._client._exec(
            "download",
            root_folder=self._root_folder,
            instance_id=instance_id,
            backup_id=backup_id,
        )
        if res.downloaded_path is None:
            raise RuntimeError("internal error: wrong result in DownloadFile")
        return res.downloaded_path

    def delete(self, instance_id: str, backup_id: str) -> None:
        self._client._exec(
            "delete",
            root_folder=self._root_folder,
            instance_id=instance_id,
            backup_id=backup_id,
        )

    def upload(
        self,
        instance_id: str,
        backup_id: str,
        *,
        name: str,
        metadata_json: str,
        content_path: str,
        max_tries: int = 0,
        chunk_size_bytes: int = 0,
    ) -> None:
        self._client._exec(
            "upload",
            root_folder=self._root_folder,
            instance_id=instance_id,
            backup_id=backup_id,
            name=name,
            metadata_json=metadata_json,
            content_path=content_path,
            upload_retries=max_tries,
            upload_chunk_size_bytes=chunk_size_bytes,
        )

    def list_files_metadata(self, instance_id: str) -> List[str]:
        res = self._client._exec(
            "list-metadata",
            root_folder=self._root_folder,
            instance_id=instance_id,
        )
        if res.metadata is None:
            raise RuntimeError("internal error: wrong result in list_files_metadata")
        return res.metadata


class Client:
    _creds: Credentials
    _on_auth_change: OnAuthChange
    _share_id: str
    _log_level: str | None

    def __init__(
        self,
        *,
        creds: Credentials,
        on_auth_change: OnAuthChange,
        log_level: str | None = None,
    ):
        self._creds = creds
        self._on_auth_change = on_auth_change
        self._share_id = ""
        self._log_level = log_level
        self._exec("check")

    def make_root_folder(self, path: str) -> Folder:
        return Folder(client=self, root_folder=path)

    def list_shares(self) -> List[Share]:
        res = self._exec("list-shares")
        if res.shares is None:
            raise RuntimeError("internal error: wrong result in list_shares")
        return res.shares

    def select_share(self, share_id: str) -> None:
        self._share_id = share_id
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
            log_level=self._log_level,
            **kwargs,
        )


def login(
    *,
    username: str,
    password: str,
    mailbox_password: str | None = None,
    mfa: str | None = None,
    log_level: str | None = None,
    captcha_token: str | None = None,
) -> Credentials:
    res = _call_go_exec(
        "login",
        username=username,
        password=password,
        mailbox_password=mailbox_password,
        mfa=mfa,
        captcha_token=captcha_token,
        log_level=log_level,
    )
    if res.creds is None:
        raise RuntimeError("internal error: wrong result in Login")
    return res.creds

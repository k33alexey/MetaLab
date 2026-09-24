#!/usr/bin/env python3
"""Extract FileStorage ZIP payloads from 1C HBK containers.

The HBK container reader is adapted from mcp-bsl-platform-help by Alexander
Kazantsev, licensed under the MIT License:
https://github.com/Desko77/mcp-bsl-platform-help-context
"""

from __future__ import annotations

import argparse
import io
import json
import struct
import zipfile
from hashlib import sha256
from pathlib import Path


def block_header(data: bytes, address: int) -> tuple[int, int, int, int]:
    position = address + 2
    data_size = int(data[position : position + 8].decode("ascii"), 16)
    position += 9
    page_size = int(data[position : position + 8].decode("ascii"), 16)
    position += 9
    next_page = int(data[position : position + 8].decode("ascii"), 16)
    position += 11
    return data_size, page_size, next_page, position


def file_body(data: bytes, address: int) -> bytes:
    data_size, page_size, next_page, start = block_header(data, address)
    result = bytearray()
    remaining = data_size
    while remaining > 0:
        size = min(page_size, remaining)
        result.extend(data[start : start + size])
        remaining -= size
        if remaining <= 0 or next_page == 0x7FFFFFFF:
            break
        _, page_size, next_page, start = block_header(data, next_page)
    return bytes(result)


def filename(data: bytes, address: int) -> str:
    position = address + 2
    payload_size = int(data[position : position + 8].decode("ascii"), 16)
    position += 9 + 40
    return data[position : position + payload_size - 24].decode(
        "utf-16-le"
    ).rstrip("\x00")


def container_entries(path: Path) -> dict[str, bytes]:
    data = path.read_bytes()
    position = 18
    payload_size = int(data[position : position + 8].decode("ascii"), 16)
    position += 9
    int(data[position : position + 8].decode("ascii"), 16)
    position += 9 + 11
    result: dict[str, bytes] = {}
    for offset in range(0, payload_size, 12):
        header_address, body_address, reserved = struct.unpack_from(
            "<iii", data, position + offset
        )
        if reserved != 0x7FFFFFFF:
            continue
        name = filename(data, header_address)
        try:
            result[name] = file_body(data, body_address)
        except (UnicodeDecodeError, ValueError):
            # Some tiny help containers contain an unused sentinel entry with
            # an invalid body address. It is not part of FileStorage.
            continue
    return result


def safe_extract(payload: bytes, output: Path) -> int:
    count = 0
    with zipfile.ZipFile(io.BytesIO(payload)) as archive:
        root = output.resolve()
        for member in archive.infolist():
            target = (output / member.filename).resolve()
            if target != root and root not in target.parents:
                raise ValueError(f"Unsafe ZIP entry: {member.filename}")
            archive.extract(member, output)
            if not member.is_dir():
                count += 1
    return count


def extract(path: Path, output: Path) -> dict[str, object]:
    entries = container_entries(path)
    payload = entries.get("FileStorage")
    if payload is None:
        raise ValueError(f"FileStorage entry not found in {path}")
    output.mkdir(parents=True, exist_ok=True)
    count = safe_extract(payload, output)
    return {
        "source": str(path),
        "source_sha256": sha256(path.read_bytes()).hexdigest(),
        "container_entries": sorted(entries),
        "extracted_file_count": count,
    }


def main() -> None:
    parser = argparse.ArgumentParser()
    parser.add_argument("hbk", type=Path)
    parser.add_argument("output", type=Path)
    parser.add_argument("--manifest", type=Path)
    args = parser.parse_args()
    manifest = extract(args.hbk, args.output)
    if args.manifest:
        args.manifest.parent.mkdir(parents=True, exist_ok=True)
        args.manifest.write_text(
            json.dumps(manifest, ensure_ascii=False, indent=2) + "\n",
            encoding="utf-8",
        )
    print(f"Extracted {manifest['extracted_file_count']} files to {args.output}")


if __name__ == "__main__":
    main()

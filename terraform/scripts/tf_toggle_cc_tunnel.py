#!/usr/bin/env -S uv run --script
# /// script
# requires-python = ">=3.12"
# dependencies = []
# ///
"""Toggle cc-tunnel module's billable resources on/off via line comments.

Usage:
    tf_toggle_cc_tunnel.py disable   # comment out targets so `terragrunt apply` destroys them
    tf_toggle_cc_tunnel.py enable    # uncomment everything we previously disabled
    tf_toggle_cc_tunnel.py status    # report which target blocks are currently disabled
    tf_toggle_cc_tunnel.py --dry-run disable   # preview without writing

Why line comments instead of /* */?
    Each disabled line becomes `# <original>`. Diffs stay line-by-line, so
    review/blame still works and Cloud Run/SQL changes don't hide behind a
    single block-comment fold.

Re-enable is matched by surrounding marker comments inserted at disable time;
running `enable` only touches lines bracketed by those markers, so any
manually-edited disabled blocks elsewhere in the tree are left alone.
"""

from __future__ import annotations

import argparse
import re
import sys
from dataclasses import dataclass
from pathlib import Path

SCRIPT_DIR = Path(__file__).resolve().parent
MODULE_DIR = SCRIPT_DIR.parent / "modules" / "cc-tunnel"

MARK_BEGIN = "# >>> tf-toggle:disabled (cc-tunnel cost-saver) >>>"
MARK_END = "# <<< tf-toggle:disabled (cc-tunnel cost-saver) <<<"

# (filename, address) — address is "<keyword>.<label>..." (e.g. "resource.google_service_account.runtime_sa")
# or "*" meaning "every top-level block in this file".
TARGETS: list[tuple[str, str]] = [
    ("auto_redeploy.tf", "module.cc_tunnel_auto_redeploy"),
    ("auto_redeploy.tf", "module.frontend_auto_redeploy"),
    #
    ("cloudsql.tf", "*"),
    #
    ("frontend.tf", "resource.google_service_account.fe_runtime_sa"),
    ("frontend.tf", "resource.google_cloud_run_v2_service.fe_cloud_run"),
    ("frontend.tf", "resource.google_cloud_run_v2_service_iam_member.fe_public_access"),
    #
    # lb.tf: keep `locals` and the global IP address (so the Cloudflare A record stays valid);
    # everything below is Cloud Run-dependent and gets destroyed.
    ("lb.tf", "resource.google_compute_region_network_endpoint_group.cc_tunnel_neg"),
    ("lb.tf", "resource.google_compute_region_network_endpoint_group.frontend_neg"),
    ("lb.tf", "resource.google_compute_backend_service.cc_tunnel_backend"),
    ("lb.tf", "resource.google_compute_backend_service.frontend_backend"),
    ("lb.tf", "resource.google_compute_url_map.lb_url_map"),
    ("lb.tf", "resource.google_compute_managed_ssl_certificate.lb_cert"),
    ("lb.tf", "resource.google_compute_target_https_proxy.lb_https_proxy"),
    ("lb.tf", "resource.google_compute_global_forwarding_rule.lb_https_forwarding_rule"),
    #
    # login_encryption.tf: keep the secret + version (re-enable reuses the same key);
    # only the IAM grant to the disabled runtime_sa goes.
    (
        "login_encryption.tf",
        "resource.google_secret_manager_secret_iam_member.cc_runtime_login_encryption_key_accessor",
    ),
    #
    ("main.tf", "resource.google_service_account.runtime_sa"),
    ("main.tf", "resource.google_cloud_run_v2_service.cloud_run"),
    ("main.tf", "resource.google_cloud_run_v2_service_iam_member.public_access"),
    ("main.tf", "resource.google_project_iam_member.cr_runtime_compute_admin"),
    #
    ("outputs.tf", "output.cc_tunnel_url"),
    ("outputs.tf", "output.frontend_url"),
    ("outputs.tf", "output.cloud_sql_instance_connection_name"),
    ("outputs.tf", "output.cloud_sql_db_name"),
    ("outputs.tf", "output.cloud_sql_database_url_secret_id"),
    ("outputs.tf", "output.lb_https_url"),
]


# --------------------------------------------------------------------------------------
# HCL block parsing
# --------------------------------------------------------------------------------------


@dataclass(frozen=True)
class Block:
    start_line: int  # 0-indexed, inclusive
    end_line: int  # 0-indexed, inclusive (line containing the closing `}`)
    address: str  # e.g. "resource.google_service_account.runtime_sa"


_IDENT_RE = re.compile(r"[A-Za-z_][A-Za-z0-9_]*")


def find_top_level_blocks(text: str) -> list[Block]:
    """Return all top-level blocks in an HCL document.

    Tracks string literals, line/block comments, and heredocs so that braces
    inside any of those don't perturb depth. Top-level == brace depth zero.
    """
    lines = text.split("\n")

    depth = 0
    in_string = False
    in_block_comment = False
    in_heredoc = False
    heredoc_term: str | None = None
    heredoc_indent = False

    block_start_line = -1
    blocks: list[Block] = []

    for line_no, line in enumerate(lines):
        if in_heredoc:
            check = line.strip() if heredoc_indent else line
            if check == heredoc_term:
                in_heredoc = False
                heredoc_term = None
                heredoc_indent = False
            continue

        n = len(line)
        i = 0
        while i < n:
            c = line[i]
            nxt = line[i + 1] if i + 1 < n else ""

            if in_block_comment:
                if c == "*" and nxt == "/":
                    in_block_comment = False
                    i += 2
                    continue
                i += 1
                continue
            if in_string:
                if c == "\\" and nxt:
                    i += 2
                    continue
                if c == '"':
                    in_string = False
                i += 1
                continue

            # Line comment: rest of line is ignored.
            if c == "#" or (c == "/" and nxt == "/"):
                break
            if c == "/" and nxt == "*":
                in_block_comment = True
                i += 2
                continue
            if c == '"':
                in_string = True
                i += 1
                continue
            if c == "<" and nxt == "<":
                j = i + 2
                indent = False
                if j < n and line[j] == "-":
                    indent = True
                    j += 1
                term: str | None = None
                advance_to = i + 1
                if j < n and line[j] == '"':
                    eq = line.find('"', j + 1)
                    if eq != -1:
                        term = line[j + 1 : eq]
                        advance_to = eq + 1
                else:
                    m = _IDENT_RE.match(line, j)
                    if m:
                        term = m.group(0)
                        advance_to = m.end()
                if term:
                    heredoc_term = term
                    heredoc_indent = indent
                    in_heredoc = True
                    # `<<EOF` must be the last meaningful token on the line; rest is whitespace.
                    break
                i = advance_to
                continue

            if c == "{":
                if depth == 0 and block_start_line < 0:
                    block_start_line = line_no
                depth += 1
                i += 1
                continue
            if c == "}":
                depth -= 1
                if depth == 0 and block_start_line >= 0:
                    address = _extract_address(lines, block_start_line)
                    blocks.append(Block(block_start_line, line_no, address))
                    block_start_line = -1
                i += 1
                continue

            if depth == 0 and not c.isspace() and block_start_line < 0:
                block_start_line = line_no

            i += 1

    return blocks


def _extract_address(lines: list[str], start_line: int) -> str:
    """Tokenize the block header (everything before the first `{`) into a dotted address."""
    pieces: list[str] = []
    for line_no in range(start_line, len(lines)):
        line = lines[line_no]
        brace = _find_brace_outside_string(line)
        if brace >= 0:
            pieces.append(line[:brace])
            break
        pieces.append(line)
    header = "\n".join(pieces)

    tokens: list[str] = []
    i = 0
    n = len(header)
    while i < n:
        c = header[i]
        if c.isspace():
            i += 1
            continue
        if c == "#":
            # comments before the brace shouldn't really happen for valid HCL,
            # but be defensive: skip rest of this header line.
            j = header.find("\n", i)
            i = j if j != -1 else n
            continue
        if c == '"':
            buf: list[str] = []
            j = i + 1
            while j < n and header[j] != '"':
                if header[j] == "\\" and j + 1 < n:
                    buf.append(header[j + 1])
                    j += 2
                    continue
                buf.append(header[j])
                j += 1
            tokens.append("".join(buf))
            i = j + 1
            continue
        m = _IDENT_RE.match(header, i)
        if m:
            tokens.append(m.group(0))
            i = m.end()
            continue
        i += 1
    return ".".join(tokens)


def _find_brace_outside_string(line: str) -> int:
    in_string = False
    i = 0
    n = len(line)
    while i < n:
        c = line[i]
        nxt = line[i + 1] if i + 1 < n else ""
        if in_string:
            if c == "\\" and nxt:
                i += 2
                continue
            if c == '"':
                in_string = False
            i += 1
            continue
        if c == "#" or (c == "/" and nxt == "/"):
            return -1
        if c == '"':
            in_string = True
            i += 1
            continue
        if c == "{":
            return i
        i += 1
    return -1


# --------------------------------------------------------------------------------------
# Disable / enable
# --------------------------------------------------------------------------------------


def _comment_line(line: str) -> str:
    return "#" if line == "" else f"# {line}"


def _wrap_disabled(lines: list[str], start: int, end: int) -> list[str]:
    """Replace lines[start..end] with marker + each line prefixed by `# `."""
    return [
        *lines[:start],
        MARK_BEGIN,
        *(_comment_line(L) for L in lines[start : end + 1]),
        MARK_END,
        *lines[end + 1 :],
    ]


def _is_already_disabled(lines: list[str], block_start: int) -> bool:
    """True if the line above this block is our begin-marker."""
    return block_start > 0 and lines[block_start - 1] == MARK_BEGIN


def disable_file(path: Path, addresses: set[str], dry_run: bool) -> int:
    """Disable matching top-level blocks in `path`. Returns blocks newly disabled."""
    text = path.read_text()
    lines = text.split("\n")
    blocks = find_top_level_blocks(text)

    select_all = "*" in addresses
    to_disable = [
        b for b in blocks if (select_all or b.address in addresses) and not _is_already_disabled(lines, b.start_line)
    ]
    if not to_disable:
        return 0

    # Apply bottom-up so earlier indices stay valid.
    new_lines = lines
    for blk in sorted(to_disable, key=lambda b: b.start_line, reverse=True):
        new_lines = _wrap_disabled(new_lines, blk.start_line, blk.end_line)

    new_text = "\n".join(new_lines)
    if not dry_run:
        path.write_text(new_text)
    for blk in to_disable:
        marker = "DRY" if dry_run else "OK "
        print(f"  [{marker}] disable  {path.name}: {blk.address or '<anonymous>'}")
    return len(to_disable)


def enable_file(path: Path, dry_run: bool) -> int:
    """Strip every MARK_BEGIN..MARK_END region in `path`, un-prefixing `# `. Returns regions restored."""
    text = path.read_text()
    lines = text.split("\n")

    out: list[str] = []
    restored = 0
    i = 0
    n = len(lines)
    while i < n:
        if lines[i] == MARK_BEGIN:
            j = i + 1
            while j < n and lines[j] != MARK_END:
                j += 1
            if j >= n:
                print(f"  [WARN] {path.name}: unterminated marker at line {i + 1}", file=sys.stderr)
                out.extend(lines[i:])
                break
            for L in lines[i + 1 : j]:
                if L.startswith("# "):
                    out.append(L[2:])
                elif L == "#":
                    out.append("")
                else:
                    # Marker present but a line inside isn't prefixed — surface it
                    # rather than silently corrupting the file.
                    print(f"  [WARN] {path.name}: line inside disabled region missing `# ` prefix: {L!r}", file=sys.stderr)
                    out.append(L)
            restored += 1
            i = j + 1
            continue
        out.append(lines[i])
        i += 1

    if restored:
        new_text = "\n".join(out)
        if not dry_run:
            path.write_text(new_text)
        marker = "DRY" if dry_run else "OK "
        print(f"  [{marker}] enable   {path.name}: {restored} region(s)")
    return restored


def status_file(path: Path) -> list[str]:
    """Return addresses (or first header line) of disabled regions in `path`."""
    text = path.read_text()
    lines = text.split("\n")
    out: list[str] = []
    i = 0
    n = len(lines)
    while i < n:
        if lines[i] == MARK_BEGIN:
            j = i + 1
            label: str | None = None
            while j < n and lines[j] != MARK_END:
                if label is None and lines[j].startswith("# ") and lines[j][2:].lstrip():
                    label = lines[j][2:].rstrip()
                j += 1
            out.append(label or "<empty>")
            i = j + 1
            continue
        i += 1
    return out


# --------------------------------------------------------------------------------------
# CLI
# --------------------------------------------------------------------------------------


def cmd_disable(dry_run: bool) -> int:
    by_file: dict[str, set[str]] = {}
    for fname, addr in TARGETS:
        by_file.setdefault(fname, set()).add(addr)

    total = 0
    print(f"Disabling cc-tunnel cost-saver targets in {MODULE_DIR}")
    for fname in sorted(by_file):
        path = MODULE_DIR / fname
        if not path.exists():
            print(f"  [SKIP] {fname}: file not found", file=sys.stderr)
            continue
        total += disable_file(path, by_file[fname], dry_run)
    print(f"\n{total} block(s) {'would be ' if dry_run else ''}disabled.")
    return 0


def cmd_enable(dry_run: bool) -> int:
    files: set[str] = {fname for fname, _ in TARGETS}
    total = 0
    print(f"Re-enabling cc-tunnel cost-saver targets in {MODULE_DIR}")
    for fname in sorted(files):
        path = MODULE_DIR / fname
        if not path.exists():
            continue
        total += enable_file(path, dry_run)
    print(f"\n{total} region(s) {'would be ' if dry_run else ''}restored.")
    return 0


def cmd_status() -> int:
    files = sorted({fname for fname, _ in TARGETS})
    any_disabled = False
    print(f"cc-tunnel cost-saver status in {MODULE_DIR}\n")
    for fname in files:
        path = MODULE_DIR / fname
        if not path.exists():
            continue
        regions = status_file(path)
        if regions:
            any_disabled = True
            print(f"  {fname}:")
            for r in regions:
                print(f"    - {r}")
    if not any_disabled:
        print("  (nothing disabled)")
    return 0


def main(argv: list[str]) -> int:
    p = argparse.ArgumentParser(description=__doc__, formatter_class=argparse.RawDescriptionHelpFormatter)
    p.add_argument("--dry-run", action="store_true", help="show what would change, don't write")
    p.add_argument("action", choices=("disable", "enable", "status"))
    args = p.parse_args(argv)

    if not MODULE_DIR.is_dir():
        print(f"error: module dir not found: {MODULE_DIR}", file=sys.stderr)
        return 2

    if args.action == "disable":
        return cmd_disable(args.dry_run)
    if args.action == "enable":
        return cmd_enable(args.dry_run)
    return cmd_status()


if __name__ == "__main__":
    sys.exit(main(sys.argv[1:]))

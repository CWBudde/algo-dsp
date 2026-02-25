#!/usr/bin/env python3
"""
extract_irs.py – Extract selected IRs from an IRLB file, resample, and save.

Reads /mnt/projekte/Code/pw_convoverb/assets/ir-library.irlib
Writes /mnt/projekte/Code/algo-dsp/web/irs.irlib
"""

import struct
import sys
import numpy as np
from scipy.signal import resample_poly
from math import gcd

SRC_PATH = "/mnt/projekte/Code/pw_convoverb/assets/ir-library.irlib"
DST_PATH = "/mnt/projekte/Code/algo-dsp/web/irs.irlib"

TARGET_RATE = 48000

WANTED_NAMES = {"Brick Wall", "Small Hall", "Large Hall", "Vocal Plate", "Large Church"}


# ---------------------------------------------------------------------------
# Low-level helpers
# ---------------------------------------------------------------------------

def read_u16(f):
    return struct.unpack("<H", f.read(2))[0]

def read_u32(f):
    return struct.unpack("<I", f.read(4))[0]

def read_u64(f):
    return struct.unpack("<Q", f.read(8))[0]

def read_f64(f):
    return struct.unpack("<d", f.read(8))[0]

def read_str(f):
    length = read_u16(f)
    return f.read(length).decode("utf-8")

def write_u16(buf, v):
    buf += struct.pack("<H", v)
    return buf

def write_u32(buf, v):
    buf += struct.pack("<I", v)
    return buf

def write_u64(buf, v):
    buf += struct.pack("<Q", v)
    return buf

def write_f64(buf, v):
    buf += struct.pack("<d", v)
    return buf

def write_str(buf, s):
    enc = s.encode("utf-8")
    buf = write_u16(buf, len(enc))
    buf += enc
    return buf

def encode_f16(samples_f32):
    """Encode float32 ndarray as IEEE 754 half-precision bytes (little-endian)."""
    f16 = samples_f32.astype(np.float16)
    return f16.tobytes()

def decode_f16_audio(raw_bytes, channels):
    """Decode f16 interleaved bytes to float32 array of shape (frames, channels)."""
    uint16_arr = np.frombuffer(raw_bytes, dtype="<u2")
    f16_arr = uint16_arr.view(np.float16)
    f32_arr = f16_arr.astype(np.float32)
    return f32_arr.reshape(-1, channels)


# ---------------------------------------------------------------------------
# IRLB reader
# ---------------------------------------------------------------------------

def read_irlb(path):
    """
    Parse an IRLB file and return a list of dicts:
        {name, category, description, tags, sample_rate, channels, audio}
    where audio is a float32 ndarray of shape (frames, channels).
    """
    irs = []

    with open(path, "rb") as f:
        # File header (18 bytes)
        magic = f.read(4)
        if magic != b"IRLB":
            raise ValueError(f"Bad magic: {magic!r}")
        version = read_u16(f)
        if version != 1:
            raise ValueError(f"Unsupported version: {version}")
        ir_count = read_u32(f)
        index_offset = read_u64(f)

        print(f"[read_irlb] version={version} ir_count={ir_count} index_offset={index_offset}")

        # Read INDEX chunk to get IR offsets and basic metadata.
        f.seek(index_offset)
        indx_magic = f.read(4)
        if indx_magic != b"INDX":
            raise ValueError(f"Expected INDX chunk, got {indx_magic!r}")
        indx_size = read_u64(f)

        entries = []
        indx_end = f.tell() + indx_size
        while f.tell() < indx_end:
            entry_offset = read_u64(f)
            entry_rate = read_f64(f)
            entry_channels = read_u32(f)
            entry_length = read_u32(f)
            entry_name = read_str(f)
            entry_category = read_str(f)
            entries.append({
                "offset": entry_offset,
                "sample_rate": entry_rate,
                "channels": entry_channels,
                "length": entry_length,
                "name": entry_name,
                "category": entry_category,
            })

        print(f"[read_irlb] found {len(entries)} index entries")

        # For each entry, read the full IR chunk (META + AUDI).
        for entry in entries:
            f.seek(entry["offset"])
            ir_magic = f.read(4)
            if ir_magic != b"IR--":
                print(f"  Warning: expected IR-- at offset {entry['offset']}, got {ir_magic!r}, skipping")
                continue
            chunk_size = read_u64(f)
            chunk_end = f.tell() + chunk_size

            meta = {}
            audio = None

            while f.tell() < chunk_end:
                sub_magic = f.read(4)
                if len(sub_magic) < 4:
                    break
                sub_size = read_u32(f)
                sub_end = f.tell() + sub_size

                if sub_magic == b"META":
                    meta["sample_rate"] = read_f64(f)
                    meta["channels"] = read_u32(f)
                    meta["length"] = read_u32(f)
                    meta["name"] = read_str(f)
                    meta["description"] = read_str(f)
                    meta["category"] = read_str(f)
                    tag_count = read_u16(f)
                    meta["tags"] = [read_str(f) for _ in range(tag_count)]
                elif sub_magic == b"AUDI":
                    raw_audio = f.read(sub_size)
                    channels = entry["channels"]
                    audio = decode_f16_audio(raw_audio, channels)
                else:
                    pass  # unknown sub-chunk, skip

                f.seek(sub_end)

            if not meta or audio is None:
                print(f"  Warning: incomplete IR chunk for {entry['name']!r}, skipping")
                continue

            irs.append({
                "name": meta.get("name", entry["name"]),
                "category": meta.get("category", entry["category"]),
                "description": meta.get("description", ""),
                "tags": meta.get("tags", []),
                "sample_rate": meta.get("sample_rate", entry["sample_rate"]),
                "channels": meta.get("channels", entry["channels"]),
                "audio": audio,  # shape (frames, channels), float32
            })
            print(f"  Loaded: {irs[-1]['name']!r:30s}  {int(irs[-1]['sample_rate'])}Hz  "
                  f"{irs[-1]['channels']}ch  {irs[-1]['audio'].shape[0]} frames")

    return irs


# ---------------------------------------------------------------------------
# Resampling
# ---------------------------------------------------------------------------

def resample_ir(audio_f32, src_rate, dst_rate):
    """Resample audio from src_rate to dst_rate using polyphase resampling."""
    if int(src_rate) == int(dst_rate):
        return audio_f32

    g = gcd(int(src_rate), int(dst_rate))
    up = dst_rate // g
    down = int(src_rate) // g

    channels = audio_f32.shape[1]
    resampled_cols = []
    for ch in range(channels):
        col = audio_f32[:, ch]
        res = resample_poly(col, up, down).astype(np.float32)
        resampled_cols.append(res)

    return np.column_stack(resampled_cols)


# ---------------------------------------------------------------------------
# IRLB writer
# ---------------------------------------------------------------------------

def build_ir_chunk(ir_dict, sample_rate, audio_f32):
    """Build the bytes for a single IR-- chunk (META + AUDI sub-chunks)."""
    channels = audio_f32.shape[1]
    length = audio_f32.shape[0]

    # --- META sub-chunk ---
    meta_body = b""
    meta_body = write_f64(meta_body, float(sample_rate))
    meta_body = write_u32(meta_body, channels)
    meta_body = write_u32(meta_body, length)
    meta_body = write_str(meta_body, ir_dict["name"])
    meta_body = write_str(meta_body, ir_dict.get("description", ""))
    meta_body = write_str(meta_body, ir_dict.get("category", ""))
    tags = ir_dict.get("tags", [])
    meta_body = write_u16(meta_body, len(tags))
    for tag in tags:
        meta_body = write_str(meta_body, tag)

    meta_chunk = b"META" + struct.pack("<I", len(meta_body)) + meta_body

    # --- AUDI sub-chunk ---
    interleaved = audio_f32.flatten(order="C")
    raw_audio = encode_f16(interleaved)
    audi_chunk = b"AUDI" + struct.pack("<I", len(raw_audio)) + raw_audio

    body = meta_chunk + audi_chunk
    return b"IR--" + struct.pack("<Q", len(body)) + body


def write_irlb(path, selected_irs):
    """
    Write a mini IRLB file with the given IRs (already resampled to TARGET_RATE).
    Each element of selected_irs is a dict with keys:
        name, category, description, tags, audio (ndarray float32, (frames, channels))
    """
    # We first build all IR chunk bytes so we know their offsets.
    ir_chunks = []
    for ir in selected_irs:
        chunk_bytes = build_ir_chunk(ir, TARGET_RATE, ir["audio"])
        ir_chunks.append(chunk_bytes)

    # File header is 18 bytes.
    HEADER_SIZE = 18

    # Lay out IR chunks starting right after the header.
    offsets = []
    pos = HEADER_SIZE
    for chunk in ir_chunks:
        offsets.append(pos)
        pos += len(chunk)

    # INDEX chunk starts at pos.
    index_offset = pos

    # Build INDEX chunk body.
    indx_body = b""
    for ir, offset in zip(selected_irs, offsets):
        audio = ir["audio"]
        indx_body = write_u64(indx_body, offset)
        indx_body = write_f64(indx_body, float(TARGET_RATE))
        indx_body = write_u32(indx_body, audio.shape[1])
        indx_body = write_u32(indx_body, audio.shape[0])
        indx_body = write_str(indx_body, ir["name"])
        indx_body = write_str(indx_body, ir.get("category", ""))

    indx_chunk = b"INDX" + struct.pack("<Q", len(indx_body)) + indx_body

    # File header.
    ir_count = len(selected_irs)
    header = b"IRLB"
    header += struct.pack("<H", 1)          # version
    header += struct.pack("<I", ir_count)   # ir_count
    header += struct.pack("<Q", index_offset)  # index_offset

    assert len(header) == HEADER_SIZE, f"Header size mismatch: {len(header)}"

    with open(path, "wb") as f:
        f.write(header)
        for chunk in ir_chunks:
            f.write(chunk)
        f.write(indx_chunk)

    print(f"[write_irlb] wrote {path}  ({ir_count} IRs, {pos + len(indx_chunk)} bytes total)")


# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------

def main():
    print(f"Reading IRs from: {SRC_PATH}")
    all_irs = read_irlb(SRC_PATH)

    print(f"\nSelecting IRs: {sorted(WANTED_NAMES)}")
    selected = [ir for ir in all_irs if ir["name"] in WANTED_NAMES]

    found_names = {ir["name"] for ir in selected}
    missing = WANTED_NAMES - found_names
    if missing:
        print(f"Warning: the following IRs were not found: {missing}")

    print(f"\nResampling {len(selected)} IRs from source rate to {TARGET_RATE} Hz...")
    for ir in selected:
        src_rate = int(ir["sample_rate"])
        orig_frames = ir["audio"].shape[0]
        if src_rate != TARGET_RATE:
            ir["audio"] = resample_ir(ir["audio"], src_rate, TARGET_RATE)
            new_frames = ir["audio"].shape[0]
            print(f"  {ir['name']!r}: {orig_frames} frames @ {src_rate} Hz  ->  "
                  f"{new_frames} frames @ {TARGET_RATE} Hz")
        else:
            print(f"  {ir['name']!r}: already at {TARGET_RATE} Hz, no resampling needed")

    print(f"\nWriting output to: {DST_PATH}")
    write_irlb(DST_PATH, selected)

    # Verify.
    import os
    size = os.path.getsize(DST_PATH)
    print(f"Output file size: {size} bytes")
    print("Done.")


if __name__ == "__main__":
    try:
        main()
    except Exception as e:
        print(f"Error: {e}", file=sys.stderr)
        sys.exit(1)

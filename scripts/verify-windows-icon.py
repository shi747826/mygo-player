#!/usr/bin/env python3
"""Verify actual PE icon payloads against the application ICO (no Windows needed)."""
import argparse
import struct
from pathlib import Path


def resources(path):
    data = Path(path).read_bytes()
    pe = struct.unpack_from('<I', data, 0x3c)[0]
    assert data[:2] == b'MZ' and data[pe:pe + 4] == b'PE\0\0'
    count, = struct.unpack_from('<H', data, pe + 6)
    optional_size, = struct.unpack_from('<H', data, pe + 20)
    optional = pe + 24
    magic, = struct.unpack_from('<H', data, optional)
    assert magic in (0x10b, 0x20b)
    resource_rva, = struct.unpack_from('<I', data, optional + (112 if magic == 0x20b else 96) + 16)
    assert resource_rva, 'PE has no resource directory'
    sections = []
    for i in range(count):
        offset = optional + optional_size + 40 * i
        virtual_size, rva, raw_size, raw = struct.unpack_from('<IIII', data, offset + 8)
        sections.append((rva, max(virtual_size, raw_size), raw))

    def file_offset(rva):
        for start, size, raw in sections:
            if start <= rva < start + size:
                return raw + rva - start
        raise AssertionError(f'Unmapped resource RVA {rva:#x}')

    base = file_offset(resource_rva)
    found = {}

    def walk(relative, keys=()):
        offset = base + relative
        named, ids = struct.unpack_from('<HH', data, offset + 12)
        for i in range(named + ids):
            name, target = struct.unpack_from('<II', data, offset + 16 + 8 * i)
            if name & 0x80000000:
                continue
            if target & 0x80000000:
                walk(target & 0x7fffffff, keys + (name,))
            else:
                rva, size = struct.unpack_from('<II', data, base + target)
                begin = file_offset(rva)
                found[keys + (name,)] = data[begin:begin + size]

    walk(0)
    return found


def verify(executable, ico, any_group=False):
    source = Path(ico).read_bytes()
    count, = struct.unpack_from('<H', source, 4)
    expected = set()
    for i in range(count):
        size, offset = struct.unpack_from('<II', source, 6 + 16 * i + 8)
        expected.add(source[offset:offset + size])
    actual = resources(executable)
    for (kind, group, language), payload in actual.items():
        if kind != 14 or (not any_group and group != 1):
            continue
        count, = struct.unpack_from('<H', payload, 4)
        images = set()
        for i in range(count):
            image_id, = struct.unpack_from('<H', payload, 6 + 14 * i + 12)
            images.add(actual[(3, image_id, language)])
        if expected <= images:
            print(f'{Path(executable).name}: icon group {group}, {count} sizes match application artwork')
            return
    raise AssertionError('Expected application icon payloads missing from PE resources')


if __name__ == '__main__':
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('executable')
    parser.add_argument('ico')
    parser.add_argument('--any-group', action='store_true', help='Allow installer-specific icon IDs')
    args = parser.parse_args()
    verify(args.executable, args.ico, args.any_group)

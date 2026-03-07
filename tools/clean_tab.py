#!/usr/bin/env python3
"""Clean ASCII guitar tabs from classtab.org and similar sources.

Extracts the first version from a multi-version tab file and normalizes
the format to work with `musiclab tab2csv`.

Cleanup steps:
  - Extract only the first transcription version
  - Remove fingering annotations (left/right hand)
  - Remove ornament markers (slides, hammer-ons, pull-offs)
  - Replace sustain markers (===) with dashes
  - Normalize string labels: e/B/G/D/A/E (top to bottom = high to low)
  - Ensure consistent e| prefix format

Usage:
  python3 tools/clean_tab.py input.tab > output.tab
  python3 tools/clean_tab.py input.tab -o output.tab
  python3 tools/clean_tab.py input.tab --title "Piece - Composer"
"""

import re
import sys
import argparse


def extract_first_version(filepath, encoding='utf-8'):
    """Extract the first tab version from a multi-version file."""
    try:
        with open(filepath, encoding=encoding) as f:
            lines = f.readlines()
    except UnicodeDecodeError:
        with open(filepath, encoding='latin-1') as f:
            lines = f.readlines()

    tab_pattern = re.compile(r'^[eEBADG]\s*[\|\-]')

    groups = []
    current_group = []
    found_first = False

    for line in lines:
        stripped = line.rstrip()
        if tab_pattern.match(stripped):
            current_group.append(stripped)
            found_first = True
        else:
            if len(current_group) == 6:
                groups.append(current_group)
            current_group = []
            # Stop at 2nd version marker
            if found_first and groups and (
                '****' in stripped
                or re.match(
                    r'.*(2nd|3rd|4th|5th)\s+version', stripped, re.I
                )
            ):
                break

    if len(current_group) == 6:
        groups.append(current_group)

    return groups


def clean_line(line):
    """Clean a single tab line to simple format."""
    match = re.match(r'^([eEBADG])', line)
    if not match:
        return line
    label = match.group(1)
    rest = line[1:]

    # Replace sustain === with ---
    rest = re.sub(r'={2,}', lambda m: '-' * len(m.group()), rest)
    # Replace ornaments between digits: 3/8, 5p4, 9h10, 3s5
    rest = re.sub(r'(\d)[/hps](\d)', r'\1-\2', rest)
    rest = rest.replace('*', '-')

    # Keep only |, digits, -
    cleaned = ''
    for ch in rest:
        if ch == '|' or ch.isdigit():
            cleaned += ch
        else:
            cleaned += '-'

    # Strip leading dashes before first |
    if cleaned and cleaned[0] == '-':
        idx = 0
        while idx < len(cleaned) and cleaned[idx] == '-':
            idx += 1
        if idx < len(cleaned) and cleaned[idx] == '|':
            cleaned = cleaned[idx:]

    # Ensure | at start
    if not cleaned.startswith('|'):
        cleaned = '|' + cleaned

    return label + cleaned


def normalize_groups(groups):
    """Clean all groups and fix string labels."""
    cleaned_groups = []
    for group in groups:
        cleaned = [clean_line(line) for line in group]

        # Fix string labels: top line should be high e, bottom = low E
        labels = [l[0] for l in cleaned]
        if labels[0] == 'E' and labels[-1] == 'E':
            cleaned[0] = 'e' + cleaned[0][1:]
        elif labels[0] == 'E' and len(labels) > 1 and labels[1] == 'B':
            cleaned[0] = 'e' + cleaned[0][1:]

        cleaned_groups.append(cleaned)
    return cleaned_groups


def format_output(groups, title=None):
    """Format cleaned groups as a tab file."""
    lines = []
    if title:
        lines.append(f'# {title}')
        lines.append('')
    for group in groups:
        for line in group:
            lines.append(line)
        lines.append('')
    return '\n'.join(lines)


def main():
    parser = argparse.ArgumentParser(
        description='Clean ASCII guitar tabs for musiclab tab2csv'
    )
    parser.add_argument('input', help='Input tab file')
    parser.add_argument('-o', '--output', help='Output file (default: stdout)')
    parser.add_argument('--title', help='Title comment for output')
    parser.add_argument(
        '--encoding', default='utf-8', help='Input encoding (default: utf-8)'
    )
    args = parser.parse_args()

    groups = extract_first_version(args.input, args.encoding)
    if not groups:
        print(f'Error: no tab sections found in {args.input}', file=sys.stderr)
        sys.exit(1)

    cleaned = normalize_groups(groups)
    output = format_output(cleaned, args.title)

    note_count = sum(
        len(re.findall(r'\d+', line[2:]))
        for group in cleaned
        for line in group
    )
    print(
        f'Extracted {len(cleaned)} sections, ~{note_count} fret entries',
        file=sys.stderr,
    )

    if args.output:
        with open(args.output, 'w') as f:
            f.write(output + '\n')
        print(f'Written to {args.output}', file=sys.stderr)
    else:
        print(output)


if __name__ == '__main__':
    main()

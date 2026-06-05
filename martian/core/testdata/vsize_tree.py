"""Test helper which keeps a parent and child process alive with allocated memory."""

import mmap
import os
import subprocess
import sys


def allocate(vmem_kb, rss_kb):
    region = mmap.mmap(-1, vmem_kb * 1024, access=mmap.ACCESS_WRITE)
    kb_string = b"01234567" * 128
    for _ in range(rss_kb):
        region.write(kb_string)
    return region


def run_child(argv):
    region = allocate(int(argv[2]), int(argv[3]))
    os.close(1)
    for _ in sys.stdin:
        pass
    region.close()


def run_parent(argv):
    parent_region = allocate(int(argv[1]), int(argv[2]))
    child = subprocess.Popen(
        [sys.executable, __file__, "child", argv[3], argv[4]],
        stdin=subprocess.PIPE,
        stdout=subprocess.PIPE,
    )
    child.stdout.read()
    child.stdout.close()
    os.close(1)
    for _ in sys.stdin:
        pass
    child.stdin.close()
    child.wait()
    parent_region.close()


def main(argv):
    if argv[1] == "child":
        run_child(argv)
    else:
        run_parent(argv)


if __name__ == "__main__":
    main(sys.argv)

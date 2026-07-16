#!/usr/bin/env python3

import os
import signal
import sys

if len(sys.argv) < 3:
    raise SystemExit("ready file and command are required")

os.setsid()
for signal_number in (signal.SIGINT, signal.SIGTERM, signal.SIGHUP):
    signal.signal(signal_number, signal.SIG_DFL)
with open(sys.argv[1], "w", encoding="ascii") as ready_file:
    ready_file.write(f"{os.getpid()}\n")
os.execvp(sys.argv[2], sys.argv[2:])

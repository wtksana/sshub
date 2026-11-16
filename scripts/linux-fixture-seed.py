"""Create a NoCloud ISO for an isolated Alpine verification guest."""

import pathlib
import shlex
import sys

import pycdlib

directory = pathlib.Path(sys.argv[1])
key = (directory / "user_ed25519.pub").read_text().strip()
script = '''#!/bin/sh
set -eu
mkdir -p /home/alpine/.ssh /home/alpine/fixture
printf '%s\\n' KEY > /home/alpine/.ssh/authorized_keys
chmod 700 /home/alpine/.ssh
chmod 600 /home/alpine/.ssh/authorized_keys
printf '%s\\n' 'SSHub Linux SFTP 中文' > '/home/alpine/fixture/Linux 中文.txt'
chmod 640 '/home/alpine/fixture/Linux 中文.txt'
chown -R alpine:alpine /home/alpine/.ssh /home/alpine/fixture
for key in /etc/ssh/ssh_host_*_key.pub; do ssh-keygen -lf "$key" > /dev/ttyS0; done
printf 'SSHUB_LINUX_READY\\n' > /dev/ttyS0
'''.replace("KEY", shlex.quote(key))
(directory / "user-data").write_text(script, encoding="utf-8", newline="\n")
(directory / "meta-data").write_text(
    "instance-id: sshub-linux-p6\nlocal-hostname: sshub-linux\n",
    encoding="utf-8",
    newline="\n",
)
iso = pycdlib.PyCdlib()
iso.new(interchange_level=3, joliet=3, vol_ident="cidata", rock_ridge="1.09")
for name, iso_path in [("user-data", "/USERDATA.;1"), ("meta-data", "/METADATA.;1")]:
    iso.add_file(str(directory / name), iso_path=iso_path, rr_name=name, joliet_path="/" + name)
iso.write(str(directory / "seed.iso"))
iso.close()

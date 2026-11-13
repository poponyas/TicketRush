#!/usr/bin/env python3
"""Preserve logs from the project's older Kafka container before adding a volume."""
import json
import pathlib
import re
import subprocess
import tempfile


def run(*args, capture=False):
    return subprocess.run(args, check=True, text=True, stdout=subprocess.PIPE if capture else None).stdout


container = run('docker', 'compose', 'ps', '-a', '-q', 'kafka', capture=True).strip()
if container:
    info = json.loads(run('docker', 'inspect', container, capture=True))[0]
    target = '/tmp/kraft-combined-logs'
    if not any(m['Destination'] == target for m in info['Mounts']):
        # Kafka's image defaults have used different log paths. Read the actual
        # generated configuration before stopping or replacing the old broker.
        environment = dict(x.split('=', 1) for x in info['Config']['Env'] if '=' in x)
        old_target = environment.get('KAFKA_LOG_DIRS', '')
        with tempfile.TemporaryDirectory(prefix='ticketrush-kafka-config-') as config_dir:
            props = pathlib.Path(config_dir) / 'server.properties'
            run('docker', 'cp', container+':/opt/kafka/config/server.properties', str(props))
            properties = props.read_text()
            match = re.search(r'^log\.dirs[ \t]*=[ \t]*(.+)$', properties, re.M)
            if not old_target and match:
                old_target = match.group(1).strip()
            metadata = re.search(r'^metadata\.log\.dir[ \t]*=[ \t]*(.+)$', properties, re.M)
            if not old_target or ',' in old_target or (metadata and metadata.group(1).strip() != old_target):
                raise SystemExit('Legacy Kafka has unsupported separate log directories. Preserve them manually before recreating the broker.')
        config = json.loads(run('docker', 'compose', 'config', '--format', 'json', capture=True))
        volume = config['volumes']['kafka_data']['name']
        run('docker', 'volume', 'create', '--label', 'com.docker.compose.project='+config['name'],
            '--label', 'com.docker.compose.volume=kafka_data', volume, capture=True)
        # Refuse to overwrite a previously populated destination.
        probe = subprocess.run(['docker', 'run', '--rm', '--user', '0', '-v', volume+':/data',
                                '--entrypoint', '/bin/sh', 'apache/kafka:4.4.0',
                                '-c', 'test -z "$(ls -A /data)"'])
        if probe.returncode:
            raise SystemExit('Kafka destination volume is not empty. Resolve the legacy log migration before recreating Kafka.')
        backup_root = pathlib.Path('tmp')
        backup_root.mkdir(exist_ok=True)
        backup = pathlib.Path(tempfile.mkdtemp(prefix='kafka-backup-', dir=backup_root)).resolve()
        if info['State']['Running']:
            run('docker', 'stop', '--time', '30', container)
        # Keep a recoverable backup in ignored tmp/, even after successful copying.
        run('docker', 'cp', container+':'+old_target+'/.', str(backup))
        run('docker', 'run', '--rm', '--user', '0', '-v', volume+':/data',
            '-v', str(backup)+':/backup:ro', '--entrypoint', '/bin/sh', 'apache/kafka:4.4.0',
            '-c', 'cp -a /backup/. /data/ && chown -R 1000:1000 /data')
        print('Preserved legacy Kafka logs in the volume and', backup)

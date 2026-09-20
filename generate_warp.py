import urllib.request
import json
import datetime
import random
import string
import time
import subprocess

def genString(stringLength):
    letters = string.ascii_letters + string.digits
    return ''.join(random.choice(letters) for i in range(stringLength))

def digitString(stringLength):
    digit = string.digits
    return ''.join((random.choice(digit) for i in range(stringLength)))

privkey = subprocess.check_output("wg genkey", shell=True).decode('utf-8').strip()
pubkey = subprocess.check_output(f"echo '{privkey}' | wg pubkey", shell=True).decode('utf-8').strip()

url = f'https://api.cloudflareclient.com/v0a{digitString(3)}/reg'

install_id = genString(22)
body = {"key": pubkey,
        "install_id": install_id,
        "fcm_token": "{}:APA91b{}".format(install_id, genString(134)),
        "tos": datetime.datetime.now().isoformat()[:-3] + "+02:00",
        "model": "Linux",
        "locale": "en_US"}

data = json.dumps(body).encode('utf8')
headers = {'Content-Type': 'application/json; charset=UTF-8',
           'User-Agent': 'okhttp/3.12.1'}
req = urllib.request.Request(url, data, headers)
try:
    response = urllib.request.urlopen(req)
    result = json.loads(response.read())
    
    conf = f"""[Interface]
PrivateKey = {privkey}
Address = {result['config']['interface']['addresses']['v4']}/32
Address = {result['config']['interface']['addresses']['v6']}/128
DNS = 1.1.1.1
MTU = 1280

[Peer]
PublicKey = {result['config']['peers'][0]['public_key']}
AllowedIPs = 0.0.0.0/0, ::/0
Endpoint = {result['config']['peers'][0]['endpoint']['host']}
"""
    with open('warp.conf', 'w') as f:
        f.write(conf)
    print("SUCCESS")
    with open('wg_priv.txt', 'w') as f:
        f.write(privkey)
except urllib.error.HTTPError as e:
    print("HTTP ERROR", e.read())
except Exception as e:
    print("ERROR", e, result)

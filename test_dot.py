import socket
import ssl
import struct
import sys

def test_dot(hostname, target_domain):
    # Construct DNS Query for target_domain (A record)
    parts = target_domain.split('.')
    qname = b''
    for p in parts:
        qname += bytes([len(p)]) + p.encode('utf-8')
    qname += b'\x00'
    
    # ID: 0xABCD, Flags: 0x0100 (Recursion Desired)
    dns_query = struct.pack("!HHHHHH", 0xABCD, 0x0100, 1, 0, 0, 0) + qname + struct.pack("!HH", 1, 1)
    tcp_payload = struct.pack("!H", len(dns_query)) + dns_query

    ctx = ssl.create_default_context()
    
    try:
        addrs = socket.getaddrinfo(hostname, 853, socket.AF_INET6, socket.SOCK_STREAM)
        ip = addrs[0][4][0]
        
        conn = socket.create_connection((ip, 853), timeout=5)
        sock = ctx.wrap_socket(conn, server_hostname=hostname)
        
        sock.sendall(tcp_payload)
        
        length_bytes = sock.recv(2)
        resp_len = struct.unpack("!H", length_bytes)[0]
        resp = sock.recv(resp_len)
        
        print("Raw Hex Response:", resp.hex())
        
        import binascii
        offset = 12 + len(qname) + 4
        ancount = struct.unpack("!H", resp[6:8])[0]
        print(f"ANCOUNT: {ancount}")
        
        if ancount > 0:
            ip_data = resp[offset+12:offset+16]
            ip_str = ".".join(map(str, ip_data))
            print(f"Extracted IP: {ip_str}")
            
    except Exception as e:
        print(f"Error: {e}")

test_dot("seal13.guard.zcdns.id", "roblox.com")

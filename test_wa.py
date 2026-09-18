import dns.query
import dns.message
import sys

def test_dot(subdomain, domain):
    print(f"--- Menguji {domain} di {subdomain} ---")
    query = dns.message.make_query(domain, dns.rdatatype.A)
    
    # We send to the IPv6 address of the server
    try:
        response = dns.query.tls(query, "2606:c700:4020:0098:1234:4321:73ab:1", port=853, server_hostname=subdomain)
        print("Response Code:", response.rcode())
        if len(response.answer) > 0:
            for answer in response.answer:
                print(answer)
        else:
            print("No answer (NODATA / NXDOMAIN)")
    except Exception as e:
        print(f"Error: {e}")

test_dot("seal13.guard.zcdns.id", "whatsapp.com")
test_dot("seal13.guard.zcdns.id", "whatsapp.net")
test_dot("seal13.guard.zcdns.id", "wa.me")
test_dot("seal13.guard.zcdns.id", "google.com")

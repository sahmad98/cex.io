import sys
sys.path.append('../types/')
from cexio import Orderbook
#import Orderbook
from socket import *
import flatbuffers

udp_ip = '127.0.0.1'
udp_port = 38201

print 'Listening on UDP'

sock = socket(AF_INET, SOCK_DGRAM)
sock.bind((udp_ip, udp_port))

while True:
    data, addr = sock.recvfrom(1024)
    buffer = bytearray(data)
    orderbook = Orderbook.Orderbook.GetRootAsOrderbook(buffer, 0)
    print 'recived msg:', orderbook.Id(), orderbook.Pair(), orderbook.Bid(), orderbook.Ask(), orderbook.Bids().Data(0)

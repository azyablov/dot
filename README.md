# Intro

This is a protype demonstrating how DNS over TLS stubby resolver can be implemented using golang.

# Background

The key RFCs describing the sujbject matter are [RFC7858](https://datatracker.ietf.org/doc/html/rfc7858)  and [RFC1035](https://datatracker.ietf.org/doc/html/rfc1035)

## Requirements analysis

### Ports

Usage of port 853 would require elevation of privilege, so for in order to use regular user we need to pickup something > 1024.

### TLS connection and privacy profile

For fast prototyping purposes we will use opportunistic privacy profile and to not specify CA certificates (use InsecureSkipVerify), 
but at the same time we have to consider it as one of the key requirements.

Code sample how to do it is commented in a source code, but later on we have to add necessary CA PEM files by specifying them as one or more acrguments.

```golang
        // // TODO: add CA
		// rootCAPEM := `<root CA cert in PEM>`

		// rootCAs := x509.NewCertPool()
		// ok := rootCAs.AppendCertsFromPEM([]byte(rootCAPEM))
		// if !ok {
		// 	panic("failed to parse root certificate")
		// }
		tlsConf := &tls.Config{
			//	RootCAs: rootCAs, // TODO
			InsecureSkipVerify: true,
		}
```
### DNS messsages and connection handling

At the first glance we it looks like pipelining of DNS messages from clear text client toward TLS based one, 
but in order to manage application scalability have to consider parsing the following attributes:
- Two-octet message length
- Two-octet ID field in DNS message header

Of course, we have to keep in mind TLS connection reuse in future it order to minimise overhead of ramping up/down endlessly TLS connections toward DNS server.
DNS message struct looks like the following one:
```golang
type DNSMsgLength uint16

type MessageID uint16

type DNSMessage struct {
	Length DNSMsgLength
	ID     MessageID
	Body   []byte
}
```

To reuse connection toward DNS server and be able to multiplex queries from the several clients, DNSMsgLength and MessageID are used.
If we design function handling messages toward TLS server as a separate one go routine, we can use channels to send/receive DNSMessage structs to/from client handlers. So finally MessageID can be used to match received message and respond to particular client handler routine, BUT MessageID is unique for the particular client that would cause a collision, so messages could be send back to client which didn't request it and not delivered to the correct one.
If server part would be multiplexing connections in order to reuse TLS session by mutiple clients, each upsteam DNS request should use MessageID and map respectively to downstream client MessageID and client subroutine/IP@.
Alternatively part of code to manage upstream TLS connection could be implemented as separate micro-service and all requests send/receiveed via gRPC from/to clients.
The future evolution of the code could be managed potentially in two different ways described above.

In the provided prototype attributes are extracted to demonstrate the way to manage the subject.

Client connection handling:
- Non-blocking other services (implemented by protoype).
- Ability to handle multiple connections (implemented by protoype).
- Gracefull closure of TCP connection (implemented by protoype).
- Handling of UDP requests (not implemented by prototype).

UDP connection handling could be implemented via listener with subsequent traction of client IP@ and MessageID.

### Performance considreations

#### Latency

Could be imporved via the following:
- TCP fastopen - RFC7413.
- TLS connection optimisation (adds anothre two RTT):
  - Out of order processing support (that's why MessageID is extracted).
  - Use of keepalive.
  - Fast connection resemption - RFC5077.

#### State

- Redice tiemouts when number of connections growing.
- Preemtive connection closure with subsequent scaling out appliction (instantiation of new PoDs):
  - Less active clients.
  - Bigger RTT.
- Collect necessary app metrics to orchestrate app and perform scale-in/out.

#### Processing of requests

- Use load-babancers for TCP connections.
- Refuse conntions (could be use as temporary measure and considered that client application is tolerant enougth).
- Collect necessary app metrics to orchestrate app and perform scale-in/out.


## Usage of prototype

By default, if not specified via ENV vars or directy via CLI args, app using port 8853!


```sh
[admin@dct0 dot]$docker build -t dot:latest .
[admin@dct0 dot]$docker run --name dot -p 853:853/tcp --rm -d dot:latest
[admin@dct0 dot]$docker container ls
CONTAINER ID   IMAGE                             COMMAND                  CREATED        STATUS        PORTS                                   NAMES
1c43f59d07cf   dot:latest                        "./dotstart.sh"          11 hours ago   Up 11 hours   0.0.0.0:853->853/tcp, :::853->853/tcp   dot
```

dig utility can be used to verify functionality:

```sh
[admin@dct0 src]$ dig @localhost -p 853 +tcp fb.com +all
; <<>> DiG 9.11.4-P2-RedHat-9.11.4-26.P2.el7_9.4 <<>> @localhost -p 853 +tcp fb.com +all
; (2 servers found)
;; global options: +cmd
;; Got answer:
;; ->>HEADER<<- opcode: QUERY, status: NOERROR, id: 12443
;; flags: qr rd ra; QUERY: 1, ANSWER: 1, AUTHORITY: 0, ADDITIONAL: 1

;; OPT PSEUDOSECTION:
; EDNS: version: 0, flags:; udp: 512
;; QUESTION SECTION:
;fb.com.                                IN      A

;; ANSWER SECTION:
fb.com.                 300     IN      A       31.13.81.36

;; Query time: 188 msec
;; SERVER: ::1#853(::1)
;; WHEN: Mon Nov 15 10:43:11 MSK 2021
;; MSG SIZE  rcvd: 51

[admin@dct0 src]$ dig @localhost -p 853 +tcp google.com +all

; <<>> DiG 9.11.4-P2-RedHat-9.11.4-26.P2.el7_9.4 <<>> @localhost -p 853 +tcp google.com +all
; (2 servers found)
;; global options: +cmd
;; Got answer:
;; ->>HEADER<<- opcode: QUERY, status: NOERROR, id: 61374
;; flags: qr rd ra; QUERY: 1, ANSWER: 6, AUTHORITY: 0, ADDITIONAL: 1

;; OPT PSEUDOSECTION:
; EDNS: version: 0, flags:; udp: 512
;; QUESTION SECTION:
;google.com.                    IN      A

;; ANSWER SECTION:
google.com.             300     IN      A       64.233.161.101
google.com.             300     IN      A       64.233.161.139
google.com.             300     IN      A       64.233.161.113
google.com.             300     IN      A       64.233.161.102
google.com.             300     IN      A       64.233.161.100
google.com.             300     IN      A       64.233.161.138

;; Query time: 132 msec
;; SERVER: ::1#853(::1)
;; WHEN: Mon Nov 15 10:43:22 MSK 2021
;; MSG SIZE  rcvd: 135

[admin@dct0 src]$ 

```



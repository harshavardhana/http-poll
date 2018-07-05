# http-poll

This is a Sample code to show how to write `http-poll` based services which can send data to clients by keeping a persistent HTTP connection. Clients can perform various actions depending on the message sent by the server, can implement a custom protocol with this approach. Following example shows how a `curl` waits for the messages indefinitely.

## Build
```
go get -u github.com/harshavardhana/http-poll
```

## Run
Run a server instance which is waiting on the client connections
```
$ ./http-poll
```

Run a client in this example we choose `curl`, it can be any http client
```
$ curl http://localhost:8888/
```


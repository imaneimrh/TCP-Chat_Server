# TCP-Chat_Server

A multi-user TCP chat server implemented in Go with room management and file transfer capabilities.

## Features

- **Multiple Client Support**: Connect many users simultaneously
- **Room Management**: Create, join, and leave chat rooms
- **User Authentication**: Secure login system
- **File Transfer**: Send files between users
- **Direct Messaging**: Private communication between users

## Getting Started

### Prerequisites

- Go 1.19 or higher

### Installation

```bash
git clone https://github.com/imaneimrh/TCP-Chat_Server.git
cd TCP-Chat_Server
go mod tidy
```

### Running the Server

```bash
go run main.go
```

The server will start listening on port 8080 by default.

### Connecting with the Test Client

```bash
go run testClient/main.go localhost 8080 username
```

## Client Commands

- `/join <room>` - Join a chat room
- `/leave <room>` - Leave a chat room
- `/create <room>` - Create a new chat room
- `/list` - List available rooms
- `/msg <user> <message>` - Send a direct message
- `/file <user> <filepath>` - Send a file to a user
- `/quit` - Exit the client

## Project Structure

- `client/` - Client handling and message processing
- `room/` - Room management implementation
- `shared/` - Common types and interfaces
- `testClient/` - Test client implementation
- `tests/` - Integration tests

## Contributors

- Developer 1: TCP Server Core & Authentication
- Developer 2: Client Interaction & Room Management

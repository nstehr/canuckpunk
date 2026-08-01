# Canuckpunk MUD-like Game
Canuckpunk is my take on a MUD-like game based on an alternative history Canada.

# Design
A go monolith. Right now the game client is served over SSH, with the architecture in place (hopefully) to be able to support other
clients with little problem. The UI is a bubbletea based TUI. Currently, the users public key is used as an identity, allowing the game to map 
an identity/key to a user selected username.

## Running locally
`make migrate` will setup the database

`make run` will get the server up and running

# Content
Content can be modelled externally as markdown files and will be rendered appropriately in the TUI. The following guidelines apply

## Content	Rendering
- Room description:	Markdown/Glamour
- Case file:	Markdown/Glamour
- Letter or report:	Markdown/Glamour
- Help documentation:	Markdown/Glamour
- Activity log entry:	Lip Gloss/plain text
- Chat or speech:	Lip Gloss/plain text
- Table-like live status:	Bubbles/Lip Gloss
- Interactive choice:	Bubble Tea component

The static content is packaged into the binary, but can be overwritten with local files. This allows to update the content without having to update the core server.

# sapple

a smol bean to move playlists (or library) from apple music to spotify. in go lang.

## instructions

1. create a spotify app in the dev portal. set the redirect url to `http://localhost:8888/callback`
2. populate the .env with the client id and secret from that portal
3. populate the .env with the apple music jwt and token (get this from headers in requests being made to amp-api.music.apple.com)
4. `go mod download`
5. `go build`
6. `./sapple run --playlist-id p.123456789 --name "new playlist name"` OR `./sapple run --library --name "apple library"`
7. click the link in the terminal. sign in.

## faq

these havent been asked as of writing but just in case they do

### searching is slow

yea i didnt do any parallelization. if you want to add this then go for it.

### but it takes a long time

okay. `screen -dmS`

### finding my creds is hard

okay. maybe ill write more here later.

### i want to do the other way around

well this is a pretty good start im sure you could add a bit and make it work.

### there's a bug

https://github.com/haileyok/sapple/pulls

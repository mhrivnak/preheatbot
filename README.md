# preheatbot

A [Telegram](https://telegram.org/) bot that can be used to turn remote relays
on and off via [preheatpi](https://github.com/mhrivnak/preheatpi/) running on a
remote Raspberry Pi.

## Account Setup

PreheatBot uses your telegram
[username](https://telegram.org/faq#usernames-and-t-me) as your identity. It is
a requirement to setup a username on your telegram account.

It is necessary for an admin to initialize your account with PreheatBot. Please
request use of PreheatBot by opening a [github
issue](https://github.com/mhrivnak/preheatbot/issues).

## Usage

The following commands can be sent to PreheatBot via private message. It does
not respond to messages other than private messages.

### Status

`/status` or `status`: returns the current on/off state for each relay that
you have registered.

### On/Off

`/on`, `on`, `/off`, `off`: will set your relay to the specified state. If you
have more than one relay registered, you will see a list of them and be able to
click or tap on one in your telegram client.

## API

[preheatpi](https://github.com/mhrivnak/preheatpi/) uses this API to know when
it should turn a relay on or off.

### Poll

The current desired state of the heater relay can be retrieved any time using
the following URL. The `version` field can be ignored unless long-polling.

`GET https://preheatbot.hrivnak.org/api/v1/users/<username>/heaters/<heaterID>`

```
HTTP/1.1 200 OK
Content-Type: application/json
Date: Tue, 29 Dec 2020 16:29:41 GMT
Content-Length: 29

{"value":"on","version":15}
```

### Long Poll

When requesting to long-poll, the API will not return a response until a command
is received by the bot to set the state. Each time a client sets the state, the
`version` field is incremented.

1. Get the current state and version as shown above
1. Make a new request with that version as shown below and a long timeout value
1. Continue making requests with new version values as responses are received

`GET https://preheatbot.hrivnak.org/api/v1/users/<username>/heaters/<heaterID>?longpoll=true&version=15`

... potentially long delay ...


```
HTTP/1.1 200 OK
Content-Type: application/json
Date: Tue, 29 Dec 2020 16:29:41 GMT
Content-Length: 29

{"value":"off","version":16}
```

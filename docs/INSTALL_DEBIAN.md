These instructions have been tested on Amazon Lightsail with Debian 12.12

Any VPS / Cloud provider with Debian available should do just fine.

Prerequisites:
* A secure IRC server with TLS and server password enabled
* A Debian VPS or Linux container on a secure host
* A fresh github account specifically for this purpose

1. sudo apt-get update -y && sudo apt-get upgrade -y && sudo apt-get install git gh make -y
2. sudo reboot
3. reconnect
4. create 'genoeg' user
5. sudo su - genoeg 
6. install go using:

```
git clone https://github.com/soyeahso/golang-tools-install-script && cd golang-tools-install-script && bash goinstall.sh && source /home/genoeg/.bashrc
```

7. mkdir -p go/src/github.com/soyeahso
8. pushd go/src/github.com/soyeahso
9. gh auth login
10. git clone https://github.com/soyeahso/hunter3.git
11. cd hunter3
12. Install Claude Code: curl -fsSL https://claude.ai/install.sh | bash
13. make mcp-register
14. make all
15. edit ~/.claude.json mcp-filesystem section to have additional path, /home/genoeg/go/src/github.com/soyeahso/hunter3 after /home/genoeg/sandbox
16. edit ~/.claude.json and fix the mcp-brave section to look like this:

```json
       "mcp-brave": {
          "type": "stdio",
          "command": "/home/genoeg/go/src/github.com/soyeahso/hunter3/dist/mcp-brave",
          "args": [
            "/home/genoeg/sandbox"
          ],
          "env": {
                  "BRAVE_API_KEY": "${BRAVE_API_KEY}"
          }
        }
```

17. run 'make claude' and type /mcp, press enter, and observe that everything is connected
18. try running hunter3 using 'make run' and then hit ctrl-c once you're stuck at hooks emitted
19. copy config.yaml to ~/.hunter3/ and edit it to point at your irc server
20. rerun 'make run', you should now be connected




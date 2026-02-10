These instructions have been tested on Amazon Lightsail with Debian 12.12

Any VPS / Cloud provider with Debian available should do just fine.

Prerequisites:
* A secure IRC server with TLS and server password enabled
* A Debian VPS or Linux container on a secure host
* A fresh github account specifically for this purpose
* A fresh email account if you want to test the gmail / imail MCP servers
* Github Copilot account

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
12. Install Github Copilot CLI: 
13. make -f Makefile.copilot mcp-config
14. make -f Makefile.copilot all [TODO audit this step]
15. run Make -f Makefile.copilot copilot-yolo
Accept the entire folder as trusted

16. TODO

[TODO how to configure brave]

17. configure an API key for brave, put it into .env as export BRAVE_API_KEY=your_api_key, and run source .env
[TODO]

19. try running hunter3 using 'make run' and then hit ctrl-c once you're stuck at hooks emitted
20. copy config.yaml to ~/.hunter3/ and edit it to point at your irc server, also change claude to copilot
21. rerun 'make run', you should now be connected


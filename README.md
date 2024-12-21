# em_webshare
- Simple and easy to use web based sharing file app

## Goal
- install an App on one device , share to all of the devices

## How to use
- before starting anything , both of the devices must be in same network
- server user : just build em_webshare and start it using
```sh
./em_webshare
```
it will host a web server on the first available port it finds , now check for your ipv4 in the preferred network in windows you can use ipconfig command .
now you can use
```sh
upload /path/to/your/file/or/dir
```
you can upload multiple files and directories , they will be awaited till user click download button , First Added First Downloaded
- client user : open your browser on http://ipv4:port of the server user , for sending choose folder or file and select press send button , for downloading what client uploaded just press download button , if server user uploads a directory applications which works like IDM cannot download the file use browsers its own Downloader


**⚠️in server side check if OS firewall is not blocking the app⚠️**

Demo UI : https://skillfulelectro.github.io/em_webshare/

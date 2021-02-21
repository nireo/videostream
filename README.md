# go videostreamer

A simple go HLS server used for video streaming. I just wanted to test how video streaming should really be done!

## How to setup

First you need to create a `videos` directory in the root of the program. Then place whatever videos in that folder.

Then you can run the program by running `go run main.go`. After that the program will format all the videos and create folders for each video. After the formatting is finished a http server will be launched where all the videos are served. You can navigate to the `/` path and then it's pretty straightforward.

## Configuration

The configuration is done in the `.env` file:

```
port=8080
resolution=1920x1080
```

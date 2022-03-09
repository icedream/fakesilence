# fakesilence

This simple tool can be used in pipes where audio data is passed to other applications to generate "fake silence" whenever digitally silent audio is passed through.

Right now it only supports signed 16-bit LE PCM-encoded audio, this is enough to support most live audio streaming configurations.

## Purpose

**tl;dr:** Digital silence is compressed "too well" by modern codecs for live streaming purposes. This tool replaces it with inaudible noise.

To explain why this tool exists in the first place, we need to look at some OGG codecs like FLAC, Vorbis and Opus where [streaming digital silence has been an issue in live streaming](https://github.com/xiph/flac/issues/90). It more often than not makes software using those codecs bug out in various ways: Suddenly you would be streaming extra silence, a lot of the non-silent audio would get dropped, or in the worst case you even experience timeouts causing streams to drop completely.

Other codecs such as MP3, AAC or PCM do not have this issue, though the disadvantages of using these codecs instead are various: MP3 and AAC are outdated lossy codecs and will introduce more artifacts than necessary for uplinks, and PCM can often not be repackaged to be streamable considering uplinks often relying on Icecast servers or MPEG-TS containers.

So the most simple workaround is obvious: Do not stream digital silence. It is possible with 16-bit audio to produce a minimal amount of noise that is inaudible to human ears in normal audio configurations. However manually adding that noise in is suboptimal: You ideally only want to insert the noise when there's actual silence on air, and who bothers to keep checking for that? And that's where this tool comes into play.

## Usage

`fakesilence` can simply be inserted in a pipe between command-line applications:

```
./source_audio.sh | fakesilence | ./encoder.sh
```

Let's fake a digitally silent audio source by feeding in `/dev/zero` to ffmpeg, then feed that audio to a separate ffmpeg instance which is supposed to do the encoding using FLAC:

```
ffmpeg -loglevel warning -re -hide_banner -channels 2 -r 44100 -f s16le -i /dev/zero -f s16le - |\
    ffmpeg -hide_banner -channels 2  -r 44100 -f s16le -i - -c:a flac -f ogg -y /dev/null
```

The statistics output looks something like this:

```
size=       2kB time=00:00:10.44 bitrate=   1.5kbits/s speed=1.11x
```

As you can see, FLAC is excessively compressing the digital silence. For a live streaming application this is bad, so let's put `fakesilence` in between like this:

```
ffmpeg -loglevel warning -re -hide_banner -channels 2 -r 44100 -f s16le -i /dev/zero -f s16le - |\
    ./fakesilence |\
    ffmpeg -hide_banner -channels 2  -r 44100 -f s16le -i - -c:a flac -f ogg -y /dev/null
```

The new output:

```
size=     204kB time=00:00:10.61 bitrate= 157.5kbits/s speed=1.12x
```

More data is now being transmitted thanks to the added audible noise, circumventing network timeout issues.

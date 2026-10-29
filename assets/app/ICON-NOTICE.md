# Hergo application icon

The application icon is an original vector illustration created for Hergo.
Its rounded ram, spiral horn, and terminal prompt acknowledge Herdr; its five
colored exclamation marks acknowledge mygo. These references describe the
design's inspiration, not official affiliation or endorsement.

Design references:

- [Herdr logo](https://github.com/herdrdev/herdr/blob/HEAD/assets/logo.png)
- [mygo wordmark](https://github.com/egoist/mygo/blob/HEAD/website/src/components/logo.tsx)

The source is `assets/app/icon.svg`. The packaged image is `assets/app/icon.png`,
rendered at 1024 x 1024 with a transparent background outside the rounded tile.
Herdr and mygo names and brand identities remain the rights of their respective
owners; the project's MIT license does not grant rights to those brands.

Regenerate the packaged image from the repository root:

```sh
resvg -w 1024 -h 1024 assets/app/icon.svg assets/app/icon.png
```

# Preact and htm

The web UI is built with [Preact](https://preactjs.com) and its hooks, and
writes its markup with [htm](https://github.com/developit/htm) in tagged
templates instead of JSX. The files here are the builds from the packages'
archives on the npm registry. They are checked in and built into `kb`, so
there is no npm, no `package.json`, no `node_modules` and no build step, and
the page never asks another server for them.

The archives were fetched with `curl` and unpacked by hand; nothing in them
was run. Neither package has dependencies or install scripts.

| File | Package | Path in the archive |
|---|---|---|
| `preact.mjs` | preact 11.0.1 | `package/dist/preact.mjs` |
| `hooks.mjs` | preact 11.0.1 | `package/hooks/dist/hooks.mjs`, changed, see below |
| `htm.module.js` | htm 3.1.1 | `package/dist/htm.module.js` |
| `LICENSE-preact.txt` | preact 11.0.1 | `package/LICENSE` |
| `LICENSE-htm.txt` | htm 3.1.1 | `package/LICENSE` |

Archives, with the checksums the registry gives for them, both checked:

- `https://registry.npmjs.org/preact/-/preact-11.0.1.tgz`,
  published 2026-10-08, integrity
  `sha512-TNNwEq++jYNHy747LziCjPRVgEwnLSOIOAw/oYcjI3Tdnubx2EXtt76I0an8PATvUWtx/DX1kFIg1eFN+O/fkw==`,
  SHA-256 `a48a17ebaa5ccae3975fee42f3a0ee3042ef68d97060b6fb01ace3fa9996405d`
- `https://registry.npmjs.org/htm/-/htm-3.1.1.tgz`,
  published 2022-04-26, integrity
  `sha512-983Vyg8NwUE7JkZ6NmOqpCZ+sh1bKv2iYTlUkzlWmA5JD2acKoxd4KVxbMmxX/85mtfdnDmTFoNKcg5DGAvxNQ==`,
  SHA-256 `2425b9bee11409177bcabc7f32e319926fc6690c1701c0b257c88bdff2d5ba90`

SHA-256 of the files here. `TestVendorFilesAreUnchanged` in
`internal/server` checks them, so a change to one of them does not go
unnoticed:

| File | SHA-256 |
|---|---|
| `preact.mjs` | `3a3201e9ccef7d30ef1273427f1b17ef8323e464b2c8d73b4fd5636af730433e` |
| `hooks.mjs` | `df45d3c5b39fc012b2397a64d4056ef06d3fbde676c9567a7c12d4d783b6bda3` |
| `htm.module.js` | `ab33dd3f38059b9be4d5f5350128eefb2356639c4e0bbe9d9e8b3ba75847e9e4` |
| `LICENSE-preact.txt` | `1fe6958409c8c257a70c587a18b6f7f412b179b456630790d30b2ec9a8e4b7d4` |
| `LICENSE-htm.txt` | `740725f7252e750af735d0028cc534970772f513331e9f68150fede8fb3ce00f` |

## The one change

The hooks import Preact by its package name, `from"preact"`, which a
browser cannot resolve without a bundler or an import map, and the page's
Content Security Policy allows no inline import map. In `hooks.mjs` that
import reads `from"./preact.mjs"` instead. Nothing else is changed. The
original file has the SHA-256
`6cb79fe9959ad185b8b86134aff47c12fd20216b24837597b19be59a133d9947`.

The last line of `preact.mjs` and `hooks.mjs` names a source map that is not
shipped; browsers only ask for it while their developer tools are open.

## Licences

- Preact: MIT License, Copyright (c) 2015-present Jason Miller, see
  [LICENSE-preact.txt](LICENSE-preact.txt).
- htm: Apache License 2.0, see [LICENSE-htm.txt](LICENSE-htm.txt). The
  package has no NOTICE file.

The licences cover these files only, not kardboard.

## Updating

Fetch the new archives from the registry, compare their checksums with the
`integrity` the registry gives for that version, unpack them outside the
repository, copy the files above, change the import in `hooks.mjs` again,
and update the versions and checksums here and in the test.

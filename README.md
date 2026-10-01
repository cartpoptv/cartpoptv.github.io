# cartpop.tv
Reviews of cartridge games

https://cartpop.tv redirects to https://cartpoptv.github.io/

| Path         | What |
|--------------|------|
| `reviews/`   | Reviews in Markdown (`tetris.md` becomes `tetris.html`) |
| `pages/`     | Landing page, about and 404 |
| `templates/` | The HTML of the pages |
| `static/`    | CSS, .js, icons |
| `engine/`    | Gopher builds the site |
| `public/`    | Actual site |

Photos and screenshots: `https://cartpoptv.github.io/images-NNNN/`

## Build

Just `go run ./engine` and it 
writes the site to `public/`.

## License

The code is public domain ([Unlicense](LICENSE)).

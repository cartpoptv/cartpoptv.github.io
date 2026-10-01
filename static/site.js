(() => {
  const lightbox = document.getElementById("lightbox");
  if (lightbox && lightbox.showModal) {
    const full = lightbox.querySelector("img");
    document.querySelectorAll(".shots img, .photo img").forEach(img => {
      img.classList.add("is-zoomable");
      img.addEventListener("click", () => {
        full.src = img.currentSrc || img.src;
        full.alt = img.alt;
        const isScreenshot = img.closest(".shots") !== null;
        full.classList.toggle("is-pixel", isScreenshot);
        if (isScreenshot) {
          const k = Math.floor(Math.min(innerWidth * 0.95 / img.naturalWidth, innerHeight * 0.95 / img.naturalHeight));
          full.style.width = img.naturalWidth * Math.max(1, k) + "px";
        } else {
          full.style.width = "";
        }
        full.decode().catch(() => {}).finally(() => lightbox.showModal());
      });
    });
    lightbox.addEventListener("click", () => lightbox.close());
  }

  document.querySelectorAll(".compare").forEach(figure => {
    const frame = figure.querySelector(".compare-frame");
    const range = figure.querySelector(".compare-range");
    const update = () => frame.style.setProperty("--pos", range.value + "%");
    figure.classList.add("is-live");
    range.addEventListener("input", update);
    update();
  });

  const root = document.documentElement;
  const giscusTheme = value => new URL(`styles/giscus${value === "auto" ? "" : "-" + value}.css`, document.baseURI).href;
  const giscus = document.querySelector('script[src="https://giscus.app/client.js"]');
  if (giscus) giscus.dataset.theme = giscusTheme(root.dataset.theme || "auto");
  const themeSwitch = document.querySelector(".theme-switch");
  if (themeSwitch) {
    const apply = value => {
      if (value === "auto") delete root.dataset.theme; else root.dataset.theme = value;
      try {
        if (value === "auto") localStorage.removeItem("theme"); else localStorage.setItem("theme", value);
      } catch (e) {}
      const frame = document.querySelector("iframe.giscus-frame");
      if (frame) frame.contentWindow.postMessage({ giscus: { setConfig: { theme: giscusTheme(value) } } }, "https://giscus.app");
    };
    const current = themeSwitch.querySelector(`input[value="${root.dataset.theme || "auto"}"]`);
    if (current) current.checked = true;
    themeSwitch.addEventListener("change", event => apply(event.target.value));
    themeSwitch.hidden = false;
  }

  const list = document.querySelector(".review-list");
  if (list && history.pushState) {
    const pills = [...document.querySelectorAll(".filters a.label")];
    const fileOf = url => new URL(url, location.href).pathname.split("/").pop() || "index.html";
    const show = file => {
      const pill = pills.find(a => fileOf(a.href) === file);
      if (!pill) return false;
      const all = pill === pills[0];
      list.querySelectorAll(".review-item").forEach(item => {
        item.hidden = !all && ![...item.querySelectorAll("a.label")].some(a => fileOf(a.href) === file);
      });
      pills.forEach(a => {
        a.classList.toggle("is-active", a === pill);
        if (a === pill) a.setAttribute("aria-current", "true"); else a.removeAttribute("aria-current");
      });
      if (pill.dataset.title) document.title = pill.dataset.title;
      return true;
    };
    document.addEventListener("click", event => {
      const link = event.target.closest("a.label");
      if (!link || event.button || event.metaKey || event.ctrlKey || event.shiftKey || event.altKey) return;
      if (!show(fileOf(link.href))) return;
      event.preventDefault();
      history.pushState(null, "", link.href);
    });
    addEventListener("popstate", () => show(fileOf(location.href)));
  }

  const logo = document.querySelector(".site-logo");
  if (logo) {
    const width = 62, height = 14, perRow = 8;
    const lit = (bytes, x, y) => x >= 0 && x < width && y >= 0 && y < height && bytes[y * perRow + (x >> 3)] & 0x80 >> (x & 7);
    const load = name => {
      const value = getComputedStyle(logo).getPropertyValue(name);
      const svg = decodeURIComponent(value.slice(value.indexOf(",") + 1, value.lastIndexOf('"')));
      const bytes = new Array(perRow * height).fill(0);
      for (const [, x, y, w] of svg.matchAll(/M(\d+) (\d+)h(\d+)/g)) {
        for (let c = +x; c < +x + +w; c++) bytes[+y * perRow + (c >> 3)] |= 0x80 >> (c & 7);
      }
      return { name, svg, bytes };
    };
    const encode = (mask, bytes) => {
      let path = "";
      for (let y = 0; y < height; y++) {
        for (let x = 0; x < width; x++) {
          if (!lit(bytes, x, y)) continue;
          const start = x;
          while (x + 1 < width && lit(bytes, x + 1, y)) x++;
          path += `M${start} ${y}h${x - start + 1}v1h-${x - start + 1}z`;
        }
      }
      return "data:image/svg+xml," + encodeURIComponent(mask.svg.replace(/d='[^']*'/, `d='${path}'`));
    };
    const masks = () => ["--mask-logo-cartpop", "--mask-logo-tv"].map(load);

    if (document.body.classList.contains("page-404")) {
      root.dataset.intro = "drop";
      const random = n => Math.floor(Math.random() * n);
      const addressLines = [[random(7), random(2)]];
      const dataLines = [[random(8), 1 + random(2)]];
      if (random(2)) addressLines.push([random(7), random(2)]);
      if (random(2)) dataLines.push([random(8), random(3)]);
      const read = bytes => bytes.map((_, i) => {
        for (const [line, high] of addressLines) i = high ? i | 1 << line : i & ~(1 << line);
        let byte = bytes[i % bytes.length];
        for (const [line, mode] of dataLines) {
          byte = (mode === 2 ? random(2) : mode) ? byte | 1 << line : byte & ~(1 << line);
        }
        return byte;
      });
      for (const mask of masks()) logo.style.setProperty(mask.name, `url("${encode(mask, read(mask.bytes))}")`);
    }

    const frames = 90;
    const sine = Array.from({ length: 256 }, (_, i) => Math.sin(i * Math.PI / 128));
    const wave = (i, amplitude) => Math.round(amplitude * sine[i & 255]);
    const bayer = [0, 2, 3, 1];
    const intros = {
      wave: (t, a) => (x, y, src) => src(x + wave(y * 18 + t * 4, 4 * a), y),
      jelly: (t, a) => (x, y, src) => src(x, y + wave(y * 24 + t * 4, 3 * a)),
      twister: (t, a) => (x, y, src) => src(x + wave(y * 10 + t * 2, 6 * a) - Math.round(48 * a ** 3), y + wave(y * 20 + t * 3 + 64, 3 * a)),
      bounce: (t, a) => {
        const h = Math.round(17 * a * Math.abs(sine[(64 + Math.round(384 * t / frames)) & 255]));
        return (x, y, src) => src(x, y + h);
      },
      scroller: (t, a) => (x, y, src) => src(x - Math.round(66 * a), y + wave(x * 8 + t * 6, 4 * a)),
      flip: (t, a) => {
        const s = sine[(64 + Math.round(384 * a)) & 255];
        return (x, y, src) => src(x, Math.round(6.5 + (y - 6.5) / s));
      },
      blinds: (t, a) => (x, y, src) => src(x + (y & 1 ? -1 : 1) * Math.round(66 * a), y),
      rotozoom: (t, a) => {
        const i = Math.round(256 * a), sin = sine[i & 255], cos = sine[(i + 64) & 255], zoom = 1 - 0.97 * a;
        return (x, y, src) => src(Math.round(30.5 + ((x - 30.5) * cos + (y - 6.5) * sin) / zoom), Math.round(6.5 + ((y - 6.5) * cos - (x - 30.5) * sin) / zoom));
      },
      curtain: t => (x, y, src) => src(x, y + Math.max(0, Math.min(17, Math.round(17 - (t - x) * 0.7)))),
      typewriter: (t, a, ends) => {
        const end = ends[Math.min(ends.length, Math.floor((t - 6) / 7) + 1) - 1] ?? -2;
        return (x, y, src) => x <= end && src(x, y);
      },
      dither: t => {
        const level = Math.min(4, Math.floor(t / 16) + 1);
        return (x, y, src) => bayer[(y & 1) * 2 + (x & 1)] < level && src(x, y);
      },
    };
    const intro = intros[root.dataset.intro];
    if (intro && !matchMedia("(prefers-reduced-motion: reduce)").matches) {
      const sources = masks();
      const used = x => sources.some(mask => [...Array(height).keys()].some(y => lit(mask.bytes, x, y)));
      const ends = [...Array(width).keys()].filter(x => used(x) && !used(x + 1));
      const lcdWidth = 70, lcdHeight = 20, slot = 21;
      const keys = new Map(), order = [], unique = [];
      for (let t = 0; t <= frames; t++) {
        const sample = intro(t, (1 - t / frames) ** 2, ends);
        const frame = sources.map(mask => {
          const src = (x, y) => lit(mask.bytes, x, y);
          const cells = [];
          for (let y = 0; y < lcdHeight; y++) {
            for (let x = 0; x < lcdWidth; x++) {
              if (sample(x - 4, y - 3, src)) cells.push(y * lcdWidth + x);
            }
          }
          return cells;
        });
        const key = frame.join("|");
        if (!keys.has(key)) {
          keys.set(key, unique.length);
          unique.push(frame);
        }
        order.push(keys.get(key));
      }
      const strips = sources.map((mask, layer) => {
        const canvas = Object.assign(document.createElement("canvas"), { width: lcdWidth, height: slot * unique.length });
        const context = canvas.getContext("2d");
        const image = context.createImageData(canvas.width, canvas.height);
        unique.forEach((frame, k) => frame[layer].forEach(cell => image.data[(k * slot * lcdWidth + cell) * 4 + 3] = 255));
        context.putImageData(image, 0, 0);
        return canvas.toDataURL();
      });
      Promise.all(strips.map(src => Object.assign(new Image(), { src }).decode().catch(() => {}))).then(() => {
        sources.forEach((mask, layer) => logo.style.setProperty(mask.name, `url("${strips[layer]}")`));
        logo.style.setProperty("--film-rows", slot * unique.length);
        logo.dataset.film = "";
        logo.dataset.live = "";
        let start, shown = 0;
        const step = now => {
          start ??= now;
          const t = Math.min(frames, Math.floor((now - start) * 0.0597));
          if (t !== shown) logo.style.setProperty("--film-y", slot * order[shown = t]);
          if (t < frames) requestAnimationFrame(step);
        };
        requestAnimationFrame(step);
      });
    }
  }
})();

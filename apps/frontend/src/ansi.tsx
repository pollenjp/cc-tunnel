import type { CSSProperties, ReactNode } from 'react';

/**
 * Lightweight ANSI escape sequence parser that converts terminal output into
 * React nodes with inline styles. The backend uses `tmux capture-pane -e` to
 * include SGR (Select Graphic Rendition) escape sequences in the captured
 * output, which this module renders as colored text.
 *
 * Supported:
 *   - Standard 16 ANSI colors (30-37, 90-97, 40-47, 100-107)
 *   - 256-color palette (38;5;N / 48;5;N)
 *   - 24-bit true color (38;2;R;G;B / 48;2;R;G;B)
 *   - Attributes: bold, dim, italic, underline, inverse, strikethrough
 *
 * Non-SGR escape sequences (cursor movement, OSC, etc.) are stripped.
 */

type Color =
  | { kind: 'indexed'; index: number }
  | { kind: 'rgb'; r: number; g: number; b: number };

interface Style {
  fg: Color | null;
  bg: Color | null;
  bold: boolean;
  dim: boolean;
  italic: boolean;
  underline: boolean;
  inverse: boolean;
  strikethrough: boolean;
}

function defaultStyle(): Style {
  return {
    fg: null,
    bg: null,
    bold: false,
    dim: false,
    italic: false,
    underline: false,
    inverse: false,
    strikethrough: false,
  };
}

function hasActiveStyle(s: Style): boolean {
  return (
    s.fg !== null ||
    s.bg !== null ||
    s.bold ||
    s.dim ||
    s.italic ||
    s.underline ||
    s.inverse ||
    s.strikethrough
  );
}

// Tokyo Night inspired 16-color palette (matches the app theme).
const BASIC_COLORS = [
  '#15161e', // 0 black
  '#f7768e', // 1 red
  '#9ece6a', // 2 green
  '#e0af68', // 3 yellow
  '#7aa2f7', // 4 blue
  '#bb9af7', // 5 magenta
  '#7dcfff', // 6 cyan
  '#a9b1d6', // 7 white
  '#414868', // 8 bright black
  '#ff7a93', // 9 bright red
  '#b9f27c', // 10 bright green
  '#ff9e64', // 11 bright yellow
  '#7da6ff', // 12 bright blue
  '#bb9af7', // 13 bright magenta
  '#0db9d7', // 14 bright cyan
  '#c0caf5', // 15 bright white
];

const CUBE_LEVELS = [0, 95, 135, 175, 215, 255];

function indexed256ToCss(i: number): string {
  if (i < 16) return BASIC_COLORS[i];
  if (i >= 232) {
    const c = 8 + (i - 232) * 10;
    return `rgb(${c},${c},${c})`;
  }
  const n = i - 16;
  const r = CUBE_LEVELS[Math.floor(n / 36)];
  const g = CUBE_LEVELS[Math.floor((n % 36) / 6)];
  const b = CUBE_LEVELS[n % 6];
  return `rgb(${r},${g},${b})`;
}

function colorToCss(c: Color): string {
  if (c.kind === 'rgb') return `rgb(${c.r},${c.g},${c.b})`;
  return indexed256ToCss(c.index);
}

function styleToCss(style: Style): CSSProperties {
  const css: CSSProperties = {};
  let fg = style.fg;
  let bg = style.bg;
  if (style.inverse) {
    [fg, bg] = [bg, fg];
  }
  if (fg) css.color = colorToCss(fg);
  if (bg) css.backgroundColor = colorToCss(bg);
  if (style.bold) css.fontWeight = 'bold';
  if (style.italic) css.fontStyle = 'italic';
  const decorations: string[] = [];
  if (style.underline) decorations.push('underline');
  if (style.strikethrough) decorations.push('line-through');
  if (decorations.length > 0) css.textDecoration = decorations.join(' ');
  if (style.dim) css.opacity = 0.7;
  return css;
}

function applySgr(style: Style, params: number[]): Style {
  const s: Style = { ...style };
  let i = 0;
  while (i < params.length) {
    const p = params[i];
    if (p === 0) {
      Object.assign(s, defaultStyle());
    } else if (p === 1) {
      s.bold = true;
    } else if (p === 2) {
      s.dim = true;
    } else if (p === 3) {
      s.italic = true;
    } else if (p === 4) {
      s.underline = true;
    } else if (p === 7) {
      s.inverse = true;
    } else if (p === 9) {
      s.strikethrough = true;
    } else if (p === 22) {
      s.bold = false;
      s.dim = false;
    } else if (p === 23) {
      s.italic = false;
    } else if (p === 24) {
      s.underline = false;
    } else if (p === 27) {
      s.inverse = false;
    } else if (p === 29) {
      s.strikethrough = false;
    } else if (p === 39) {
      s.fg = null;
    } else if (p === 49) {
      s.bg = null;
    } else if (p === 38 || p === 48) {
      const target: 'fg' | 'bg' = p === 38 ? 'fg' : 'bg';
      const mode = params[i + 1];
      if (mode === 5 && i + 2 < params.length) {
        s[target] = { kind: 'indexed', index: params[i + 2] & 0xff };
        i += 2;
      } else if (mode === 2 && i + 4 < params.length) {
        s[target] = {
          kind: 'rgb',
          r: params[i + 2] & 0xff,
          g: params[i + 3] & 0xff,
          b: params[i + 4] & 0xff,
        };
        i += 4;
      }
    } else if (p >= 30 && p <= 37) {
      s.fg = { kind: 'indexed', index: p - 30 };
    } else if (p >= 40 && p <= 47) {
      s.bg = { kind: 'indexed', index: p - 40 };
    } else if (p >= 90 && p <= 97) {
      s.fg = { kind: 'indexed', index: p - 90 + 8 };
    } else if (p >= 100 && p <= 107) {
      s.bg = { kind: 'indexed', index: p - 100 + 8 };
    }
    i++;
  }
  return s;
}

// Matches:
//   - CSI sequences:    ESC [ params intermediates final   (final in @-~)
//   - OSC sequences:    ESC ] ... BEL  or  ESC ] ... ESC \
//   - Simple ESC:       ESC single-byte in [@-Z\-_]
// eslint-disable-next-line no-control-regex
const ESC_RE = /\x1b(?:\[([0-9;?]*)([@-~])|\][^\x07\x1b]*?(?:\x07|\x1b\\)|[@-Z\\-_])/g;

export function parseAnsi(text: string): ReactNode[] {
  if (!text) return [];
  const nodes: ReactNode[] = [];
  let style = defaultStyle();
  let last = 0;
  let key = 0;

  const pushSegment = (segment: string) => {
    if (!segment) return;
    if (hasActiveStyle(style)) {
      nodes.push(
        <span key={key++} style={styleToCss(style)}>
          {segment}
        </span>,
      );
    } else {
      nodes.push(segment);
    }
  };

  ESC_RE.lastIndex = 0;
  let m: RegExpExecArray | null;
  while ((m = ESC_RE.exec(text)) !== null) {
    if (m.index > last) {
      pushSegment(text.slice(last, m.index));
    }
    const csiParams = m[1];
    const csiFinal = m[2];
    if (csiFinal === 'm' && csiParams !== undefined) {
      const params =
        csiParams === ''
          ? [0]
          : csiParams.split(';').map((p) => {
              const n = Number(p);
              return Number.isFinite(n) ? n : 0;
            });
      style = applySgr(style, params);
    }
    // Non-SGR sequences are dropped.
    last = ESC_RE.lastIndex;
  }
  if (last < text.length) {
    pushSegment(text.slice(last));
  }
  return nodes;
}

import { Position, Rect } from "../types";

export const pointsOfRect = (rect: Rect): Position[] => {
  const { x, y, width, height } = rect;
  const nw = { x, y };
  const ne = { x: x + width, y };
  const sw = { x, y: y + height };
  const se = { x: x + width, y: y + height };
  return [nw, ne, sw, se];
};

export const calcBBox = (points: Position[]): Rect => {
  const min: Position = { x: Number.MAX_VALUE, y: Number.MAX_VALUE };
  const max: Position = { x: Number.MIN_VALUE, y: Number.MIN_VALUE };
  points.forEach(({ x, y }) => {
    if (x > max.x) max.x = x;
    if (y > max.y) max.y = y;
    if (x < min.x) min.x = x;
    if (y < min.y) min.y = y;
  });
  return {
    x: min.x,
    y: min.y,
    width: max.x - min.x,
    height: max.y - min.y,
  };
};

export type SegmentOverlap1D =
  | "BEFORE"
  | "OVERLAPS"
  | "CONTAINS"
  | "OVERLAPPED"
  | "CONTAINED"
  | "AFTER";

export const segmentOverlap1D = (
  a: number,
  b: number,
  c: number,
  d: number
): SegmentOverlap1D => {
  /**
   * Giving two segments `AB` and `CD` on a line,
   * find the intersection relationship of them.
   * 1. A-B C-D -> AB before CD
   * 2. A-C-B-D -> AB overlaps CD
   * 3. A-C-D-B -> AB contains CD
   * 4. C-A-D-B -> AB overlapped by CD
   * 5. C-A-B-D -> AB contained by CD
   * 6. C-D A-B -> AB after CD
   */
  console.assert(a <= b, `expected a=${a} to < b=${b}`);
  console.assert(c <= d, `expected c=${c} to < d=${d}`);

  if (b < c) return "BEFORE";
  if (a < c && b >= c && b < d) return "OVERLAPS";
  if (a < c && d < b) return "CONTAINS";
  if (a >= c && a < d && b >= d) return "OVERLAPPED";
  if (a >= c && b < d) return "CONTAINED";
  if (a >= d) return "AFTER";

  throw new Error(`should never be here a=${a}, b=${b}, c=${c}, d=${d}`);
};

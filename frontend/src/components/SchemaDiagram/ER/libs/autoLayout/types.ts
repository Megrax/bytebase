import type {
  Path,
  Position,
  Rect,
  Size,
} from "@/components/SchemaDiagram/types";

export type GraphChildNodeItem = { id: string; size: Size; pos: Position };

export type GraphNodeItem = {
  id: string;
  size: Size;
  children: GraphChildNodeItem[]; // not used yet
};

export type GraphEdgeItem = {
  id: string;
  from: string;
  to: string;
};

export type Layout = {
  rects: Map<string, Rect>;
  paths: Map<string, Path>;
};

#!/usr/bin/env python3
"""
Sprout Lands sprite sheet slicer for 农趣村 (NongQuCun).

Reads raw sprite sheets from "Sprout Lands/" directory, generates
PixiJS-compatible JSON atlas files alongside organized PNG copies
in "client/src/assets/sprites/".

Usage:
    python3 scripts/slice_sprites.py

Output per sprite sheet:
    - <name>.png   (copy of source, renamed)
    - <name>.json  (PixiJS Spritesheet JSON Hash format)
"""

import json
import os
import shutil
from dataclasses import dataclass, field
from pathlib import Path

try:
    from PIL import Image
except ImportError:
    print("ERROR: Pillow is required. Install with: pip3 install Pillow")
    raise SystemExit(1)

# ---------------------------------------------------------------------------
# Config
# ---------------------------------------------------------------------------

PROJECT_ROOT = Path(__file__).resolve().parent.parent
SPROUT_DIR = PROJECT_ROOT / "Sprout Lands"
ASSETS_DIR = PROJECT_ROOT / "client" / "src" / "assets" / "sprites"


@dataclass
class SpriteSheet:
    """Defines how to slice a single sprite sheet."""
    src: str                          # Relative path under SPROUT_DIR
    dst: str                          # Relative path under ASSETS_DIR (no ext)
    frame_w: int                      # Frame width in pixels
    frame_h: int                      # Frame height in pixels
    prefix: str = ""                  # Frame name prefix (auto-derived if empty)
    animations: dict = field(default_factory=dict)  # name -> [frame_indices]


# -- Characters ---------------------------------------------------------------

CHARACTER_WALK = SpriteSheet(
    src="Characters/Basic Charakter Spritesheet.png",
    dst="characters/player-walk",
    frame_w=48, frame_h=48,
    prefix="player",
    animations={
        "walk-down":  [0, 1, 2, 3],
        "walk-up":    [4, 5, 6, 7],
        "walk-left":  [8, 9, 10, 11],
        "walk-right": [12, 13, 14, 15],
    },
)

CHARACTER_ACTIONS = SpriteSheet(
    src="Characters/Basic Charakter Actions.png",
    dst="characters/player-actions",
    frame_w=48, frame_h=48,
    prefix="action",
    animations={
        # 96 wide / 48 = 2 cols,  576 high / 48 = 12 rows
        # Rows grouped by action type (2 frames each, left + right variant)
        "axe-right":       [0, 1],
        "axe-left":        [2, 3],
        "axe-down":        [4, 5],
        "axe-up":          [6, 7],
        "water-right":     [8, 9],
        "water-left":      [10, 11],
        "water-down":      [12, 13],
        "water-up":        [14, 15],
        "hoe-right":       [16, 17],
        "hoe-left":        [18, 19],
        "hoe-down":        [20, 21],
        "hoe-up":          [22, 23],
    },
)

CHARACTER_TOOLS = SpriteSheet(
    src="Characters/Tools.png",
    dst="items/tools-anim",
    frame_w=16, frame_h=16,
    prefix="tool",
)

# -- Animals ------------------------------------------------------------------

CHICKEN = SpriteSheet(
    src="Characters/Free Chicken Sprites.png",
    dst="animals/chicken",
    frame_w=16, frame_h=16,
    prefix="chicken",
    animations={
        "idle": [0, 1, 2, 3],
        "walk": [4, 5, 6, 7],
    },
)

COW = SpriteSheet(
    src="Characters/Free Cow Sprites.png",
    dst="animals/cow",
    frame_w=32, frame_h=32,
    prefix="cow",
    animations={
        "idle": [0, 1, 2],
        "walk": [3, 4, 5],
    },
)

EGG_NEST = SpriteSheet(
    src="Characters/Egg_And_Nest.png",
    dst="animals/egg-nest",
    frame_w=16, frame_h=16,
    prefix="egg-nest",
)

CHICKEN_HOUSE = SpriteSheet(
    src="Objects/Free_Chicken_House.png",
    dst="buildings/chicken-house",
    frame_w=48, frame_h=48,
    prefix="chicken-house",
)

# -- Terrain / Tilesets -------------------------------------------------------

GRASS = SpriteSheet(
    src="Tilesets/Grass.png",
    dst="terrain/grass",
    frame_w=16, frame_h=16,
    prefix="grass",
)

TILLED_DIRT = SpriteSheet(
    src="Tilesets/Tilled_Dirt_v2.png",
    dst="terrain/tilled-dirt",
    frame_w=16, frame_h=16,
    prefix="dirt",
)

TILLED_DIRT_WIDE = SpriteSheet(
    src="Tilesets/Tilled_Dirt_Wide_v2.png",
    dst="terrain/tilled-dirt-wide",
    frame_w=16, frame_h=16,
    prefix="dirt-wide",
)

WATER = SpriteSheet(
    src="Tilesets/Water.png",
    dst="terrain/water",
    frame_w=16, frame_h=16,
    prefix="water",
)

HILLS = SpriteSheet(
    src="Tilesets/Hills.png",
    dst="terrain/hills",
    frame_w=16, frame_h=16,
    prefix="hill",
)

FENCES = SpriteSheet(
    src="Tilesets/Fences.png",
    dst="terrain/fences",
    frame_w=16, frame_h=16,
    prefix="fence",
)

PATHS = SpriteSheet(
    src="Objects/Paths.png",
    dst="terrain/paths",
    frame_w=16, frame_h=16,
    prefix="path",
)

DOORS = SpriteSheet(
    src="Tilesets/Doors.png",
    dst="buildings/doors",
    frame_w=16, frame_h=16,
    prefix="door",
)

WOODEN_HOUSE_ROOF = SpriteSheet(
    src="Tilesets/Wooden_House_Roof_Tilset.png",
    dst="buildings/wooden-house-roof",
    frame_w=16, frame_h=16,
    prefix="roof",
)

WOODEN_HOUSE_WALLS = SpriteSheet(
    src="Tilesets/Wooden_House_Walls_Tilset.png",
    dst="buildings/wooden-house-walls",
    frame_w=16, frame_h=16,
    prefix="wall",
)

# -- Objects ------------------------------------------------------------------

PLANTS = SpriteSheet(
    src="Objects/Basic_Plants.png",
    dst="objects/plants",
    frame_w=16, frame_h=16,
    prefix="plant",
)

FURNITURE = SpriteSheet(
    src="Objects/Basic_Furniture.png",
    dst="objects/furniture",
    frame_w=16, frame_h=16,
    prefix="furn",
)

GRASS_BIOME = SpriteSheet(
    src="Objects/Basic_Grass_Biom_things.png",
    dst="objects/grass-biome",
    frame_w=16, frame_h=16,
    prefix="biome",
)

CHEST = SpriteSheet(
    src="Objects/Chest.png",
    dst="objects/chest",
    frame_w=24, frame_h=24,
    prefix="chest",
    animations={
        "closed-front": [0, 1, 2, 3, 4],
        "closed-side":  [5, 6, 7, 8, 9],
    },
)

BRIDGE = SpriteSheet(
    src="Objects/Wood_Bridge.png",
    dst="objects/bridge",
    frame_w=16, frame_h=16,
    prefix="bridge",
)

TOOLS_ITEMS = SpriteSheet(
    src="Objects/Basic_tools_and_meterials.png",
    dst="items/tools",
    frame_w=16, frame_h=16,
    prefix="item-tool",
)

MILK_GRASS = SpriteSheet(
    src="Objects/Simple_Milk_and_grass_item.png",
    dst="items/milk-grass",
    frame_w=16, frame_h=16,
    prefix="item",
)

EGG_ITEM = SpriteSheet(
    src="Objects/Egg_item.png",
    dst="items/egg",
    frame_w=16, frame_h=16,
    prefix="egg",
)

# -- All sheets to process ----------------------------------------------------

ALL_SHEETS = [
    CHARACTER_WALK, CHARACTER_ACTIONS, CHARACTER_TOOLS,
    CHICKEN, COW, EGG_NEST, CHICKEN_HOUSE,
    GRASS, TILLED_DIRT, TILLED_DIRT_WIDE, WATER, HILLS, FENCES, PATHS,
    DOORS, WOODEN_HOUSE_ROOF, WOODEN_HOUSE_WALLS,
    PLANTS, FURNITURE, GRASS_BIOME, CHEST, BRIDGE,
    TOOLS_ITEMS, MILK_GRASS, EGG_ITEM,
]


# ---------------------------------------------------------------------------
# Atlas Generator
# ---------------------------------------------------------------------------

def generate_atlas(sheet: SpriteSheet) -> dict:
    """Generate PixiJS Spritesheet JSON Hash format atlas."""
    src_path = SPROUT_DIR / sheet.src
    if not src_path.exists():
        print(f"  SKIP (not found): {sheet.src}")
        return None

    img = Image.open(src_path)
    img_w, img_h = img.size

    cols = img_w // sheet.frame_w
    rows = img_h // sheet.frame_h

    if cols == 0 or rows == 0:
        print(f"  SKIP (frame too large): {sheet.src} "
              f"({img_w}x{img_h}, frame {sheet.frame_w}x{sheet.frame_h})")
        return None

    # Build frames
    frames = {}
    frame_idx = 0
    for row in range(rows):
        for col in range(cols):
            x = col * sheet.frame_w
            y = row * sheet.frame_h

            # Skip fully transparent frames
            region = img.crop((x, y, x + sheet.frame_w, y + sheet.frame_h))
            if region.mode == "RGBA":
                # Check if entire frame is transparent
                alpha = region.split()[3]
                if alpha.getbbox() is None:
                    frame_idx += 1
                    continue

            name = f"{sheet.prefix}-{frame_idx:03d}"
            frames[name] = {
                "frame": {"x": x, "y": y, "w": sheet.frame_w, "h": sheet.frame_h},
                "rotated": False,
                "trimmed": False,
                "spriteSourceSize": {"x": 0, "y": 0, "w": sheet.frame_w, "h": sheet.frame_h},
                "sourceSize": {"w": sheet.frame_w, "h": sheet.frame_h},
            }
            frame_idx += 1

    # Build animations (map name -> list of frame keys)
    animations = {}
    if sheet.animations:
        for anim_name, indices in sheet.animations.items():
            anim_frames = []
            for idx in indices:
                key = f"{sheet.prefix}-{idx:03d}"
                if key in frames:
                    anim_frames.append(key)
            if anim_frames:
                animations[anim_name] = anim_frames

    # Build atlas
    png_filename = Path(sheet.dst).name + ".png"
    atlas = {
        "frames": frames,
        "animations": animations if animations else undefined_skip(),
        "meta": {
            "app": "nongqucun-sprite-slicer",
            "version": "1.0.0",
            "image": png_filename,
            "format": "RGBA8888",
            "size": {"w": img_w, "h": img_h},
            "scale": "1",
        },
    }

    # Remove empty animations key
    if not animations:
        del atlas["animations"]

    return atlas


def undefined_skip():
    """Placeholder that gets removed."""
    return {}


def process_sheet(sheet: SpriteSheet) -> bool:
    """Process one sprite sheet: copy PNG + generate atlas JSON."""
    src_path = SPROUT_DIR / sheet.src
    dst_png = ASSETS_DIR / (sheet.dst + ".png")
    dst_json = ASSETS_DIR / (sheet.dst + ".json")

    if not src_path.exists():
        print(f"  SKIP: {sheet.src} (file not found)")
        return False

    # Ensure output directory exists
    dst_png.parent.mkdir(parents=True, exist_ok=True)

    # Copy PNG
    shutil.copy2(src_path, dst_png)

    # Generate atlas
    atlas = generate_atlas(sheet)
    if atlas is None:
        return False

    # Write JSON
    with open(dst_json, "w", encoding="utf-8") as f:
        json.dump(atlas, f, indent=2, ensure_ascii=False)

    frame_count = len(atlas["frames"])
    anim_count = len(atlas.get("animations", {}))
    anim_info = f", {anim_count} animations" if anim_count else ""
    print(f"  OK: {sheet.dst}.json ({frame_count} frames{anim_info})")
    return True


def copy_palette():
    """Copy color palette file."""
    src = SPROUT_DIR / "Sprout Lands color pallet" / "Sprout Lands defautlt palette.png"
    dst = ASSETS_DIR / "palette" / "sprout-lands-palette.png"
    dst.parent.mkdir(parents=True, exist_ok=True)
    if src.exists():
        shutil.copy2(src, dst)
        print(f"  OK: palette/sprout-lands-palette.png")
    # Also copy the aseprite file if present
    src_ase = src.with_suffix(".aseprite")
    if src_ase.exists():
        shutil.copy2(src_ase, dst.with_suffix(".aseprite"))
        print(f"  OK: palette/sprout-lands-palette.aseprite")


def generate_index(processed: list[str]):
    """Generate a TypeScript barrel file listing all atlas paths."""
    index_path = ASSETS_DIR / "index.ts"
    lines = [
        "/**",
        " * Auto-generated sprite atlas index.",
        " * Run `python3 scripts/slice_sprites.py` to regenerate.",
        " *",
        " * Assets from: Sprout Lands by Cup Nooble",
        " * License: Non-commercial (contact cup_nooble on Discord for commercial)",
        " */",
        "",
    ]

    # Group by directory
    groups: dict[str, list[str]] = {}
    for p in sorted(processed):
        category = p.split("/")[0]
        groups.setdefault(category, []).append(p)

    for category, paths in sorted(groups.items()):
        const_name = category.upper().replace("-", "_")
        lines.append(f"// {category}")
        for p in paths:
            var_name = (
                Path(p).stem
                .replace("-", "_")
                .replace(" ", "_")
                .upper()
            )
            lines.append(f"export const ATLAS_{var_name} = "
                         f"require('./{p}.json');")
            lines.append(f"export const IMG_{var_name} = "
                         f"require('./{p}.png');")
        lines.append("")

    with open(index_path, "w", encoding="utf-8") as f:
        f.write("\n".join(lines))
    print(f"\n  Generated: {index_path.relative_to(PROJECT_ROOT)}")


# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------

def main():
    print("=" * 60)
    print("Sprout Lands Sprite Slicer for 农趣村")
    print("=" * 60)
    print(f"\nSource:  {SPROUT_DIR}")
    print(f"Output:  {ASSETS_DIR}\n")

    if not SPROUT_DIR.exists():
        print(f"ERROR: Sprout Lands directory not found at {SPROUT_DIR}")
        raise SystemExit(1)

    processed = []
    skipped = 0

    for sheet in ALL_SHEETS:
        if process_sheet(sheet):
            processed.append(sheet.dst)
        else:
            skipped += 1

    print()
    copy_palette()
    generate_index(processed)

    print(f"\n{'=' * 60}")
    print(f"Done! Processed {len(processed)} sprite sheets, skipped {skipped}")
    print(f"Output: {ASSETS_DIR.relative_to(PROJECT_ROOT)}/")
    print(f"{'=' * 60}")


if __name__ == "__main__":
    main()

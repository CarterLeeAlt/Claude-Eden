"""Build private Windows UI faces from the bundled Inter and Noto Sans SC.

Requires FontTools 4.68.0. Run with a workspace Python and PYTHONPATH pointing
to its isolated dependencies. Inputs and output are in assets/fonts; temporary
static instances are confined to .tmp/native-fonts. Web uses the original faces.
"""
from pathlib import Path
import hashlib
import json

from fontTools import __version__
from fontTools.merge import Merger
from fontTools.merge.options import Options as MergeOptions
from fontTools.subset import Options, Subsetter
from fontTools.ttLib import TTFont
from fontTools.ttLib.scaleUpem import scale_upem
from fontTools.varLib.instancer import instantiateVariableFont

ROOT = Path(__file__).resolve().parents[1]
FONTS = ROOT / "assets/fonts"
WORK = ROOT / ".tmp/native-fonts"


def main():
    if __version__ != "4.68.0":
        raise RuntimeError("Use FontTools 4.68.0 for reproducible font output")
    WORK.mkdir(parents=True, exist_ok=True)
    records = []
    for weight, style in [(400, "Regular"), (700, "Bold")]:
        print(f"Building Eden UI {style}: Inter + Noto Sans SC", flush=True)
        paths = []
        latin = set()
        expected = {}
        copyrights = []
        for family, filename in [("Inter", "Inter-Variable.ttf"), ("Noto Sans SC", "NotoSansSC-Variable.ttf")]:
            with TTFont(FONTS / filename, recalcTimestamp=False) as source:
                copyrights.append(source["name"].getDebugName(0))
                font = instantiateVariableFont(source, {"wght": weight}, static=True)
            scale_upem(font, 1000)
            # Keep Inter's characters as the first face of the web font stack.
            points = set(font.getBestCmap()) - latin
            if family == "Inter":
                latin = points
            options = Options()
            options.hinting = False
            options.name_IDs = ["*"]
            subsetter = Subsetter(options=options)
            subsetter.populate(unicodes=points)
            subsetter.subset(font)
            for point, glyph in font.getBestCmap().items():
                expected[point] = font["hmtx"][glyph][0]
            path = WORK / f"{family.replace(' ', '-')}-{style}.ttf"
            font.save(path)
            paths.append(str(path))
            font.close()
        # WinForms/NSIS use horizontal text; Inter has no vertical metrics.
        merged = Merger(options=MergeOptions(drop_tables=["vhea", "vmtx", "VORG", "STAT"])).merge(paths)
        # The derived family is renamed in accordance with the SIL OFL.
        names = merged["name"]
        names.names = []
        for name_id, value in {
            0: "\n".join(copyrights),
            1: "Eden UI", 2: style, 3: f"EdenUI-{style}-1.0",
            4: f"Eden UI {style}", 5: "Version 1.000",
            6: f"EdenUI-{style}", 16: "Eden UI", 17: style,
            13: "SIL Open Font License, Version 1.1. See Inter-LICENSE.txt and Noto-LICENSE.txt.",
            14: "https://openfontlicense.org/",
        }.items():
            names.setName(value, name_id, 3, 1, 0x409)
        merged["OS/2"].usWeightClass = weight
        merged["OS/2"].fsSelection = 0x20 if weight == 700 else 0x40
        merged["head"].macStyle = 1 if weight == 700 else 0
        merged["head"].created = merged["head"].modified = 3800000000
        merged.recalcTimestamp = False
        output = FONTS / f"EdenUI-{style}.ttf"
        merged.save(output)
        merged.close()
        with TTFont(output) as result:
            cmap = result.getBestCmap()
            if set(cmap) != set(expected):
                raise RuntimeError(f"Character coverage changed in {output.name}")
            for point, advance in expected.items():
                if result["hmtx"][cmap[point]][0] != advance:
                    raise RuntimeError(f"Character advance changed: U+{point:04X}")
            records.append({"file": output.name, "characters": len(cmap), "sha256": hashlib.sha256(output.read_bytes()).hexdigest()})
    print(json.dumps(records, indent=2))


if __name__ == "__main__":
    main()

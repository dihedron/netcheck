-- include.lua replaces a code block marked {include="path"} with the
-- content of the file at path (relative to the repository root), so that
-- the documents show the tested samples rather than copies of them:
--
--     ```{.yaml include="examples/signer.yaml"}
--     ```
--
-- A missing file fails the build.
function CodeBlock(block)
  local path = block.attributes.include
  if not path then
    return nil
  end
  local file = io.open(path, "r")
  if not file then
    error("include: cannot read " .. path)
  end
  block.text = file:read("a"):gsub("\n$", "")
  file:close()
  block.attributes.include = nil
  return block
end

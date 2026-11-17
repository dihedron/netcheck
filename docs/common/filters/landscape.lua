-- landscape.lua puts the content of a div with the class landscape on
-- pages of their own, turned to landscape, for a table or a diagram that
-- does not fit in portrait (lscape, loaded with the rotation flag in
-- defaults.yaml):
--
--     ::: landscape
--     | wide | table |
--     :::
function Div(div)
  if not div.classes:includes("landscape") or not FORMAT:match("latex") then
    return nil
  end
  local blocks = pandoc.List({ pandoc.RawBlock("latex", "\\begin{landscape}\\global\\landscapepagetrue") })
  blocks:extend(div.content)
  blocks:insert(pandoc.RawBlock("latex", "\\end{landscape}\\global\\landscapepagefalse"))
  return blocks
end

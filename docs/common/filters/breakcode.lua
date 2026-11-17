-- breakcode.lua lets LaTeX break inline code after "_", ".", "/", "-"
-- and ":", so that configuration keys, paths and selectors in narrow
-- table columns wrap instead of running into the next column.
local escapes = {
  ["\\"] = "\\textbackslash{}",
  ["{"] = "\\{",
  ["}"] = "\\}",
  ["$"] = "\\$",
  ["&"] = "\\&",
  ["#"] = "\\#",
  ["^"] = "\\textasciicircum{}",
  ["_"] = "\\_",
  ["%"] = "\\%",
  ["~"] = "\\textasciitilde{}",
}
local breaks = { ["_"] = true, ["."] = true, ["/"] = true, ["-"] = true, [":"] = true }

function Code(code)
  if not FORMAT:match("latex") then
    return nil
  end
  local out = {}
  for _, c in utf8.codes(code.text) do
    local ch = utf8.char(c)
    out[#out + 1] = escapes[ch] or ch
    if breaks[ch] then
      out[#out + 1] = "\\allowbreak{}"
    end
  end
  return pandoc.RawInline("latex", "\\texttt{" .. table.concat(out) .. "}")
end

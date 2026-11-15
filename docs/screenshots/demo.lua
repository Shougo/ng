-- Minimal config for the README screenshots. See README.md in this
-- directory for how each screenshot is taken.
--
--   NG_DEMO=edit ng -- --clean -u docs/screenshots/demo.lua
--
-- NG_DEMO is one of edit (the default), ime or image.
-- NG_DEMO_FONT overrides 'guifont' and NG_DEMO_SIZE (COLSxROWS) the size.

local dir = vim.fn.fnamemodify(debug.getinfo(1, 'S').source:sub(2), ':p:h')
local root = vim.fn.fnamemodify(dir, ':h:h')
local scene = vim.env.NG_DEMO or 'edit'

local sizes = { edit = '100x30', ime = '72x20', image = '80x24' }
local cols, rows = (vim.env.NG_DEMO_SIZE or sizes[scene] or '80x24'):match('^(%d+)x(%d+)$')

vim.o.guifont = vim.env.NG_DEMO_FONT or 'UDEV Gothic NF:h12'
vim.o.lines = tonumber(rows)
vim.o.columns = tonumber(cols)
vim.o.number = true
vim.o.cursorline = true
vim.o.laststatus = 2
vim.o.shortmess = vim.o.shortmess .. 'I'
vim.o.swapfile = false

local function icon(nr)
  return vim.fn.nr2char(nr)
end

-- A floating window with Nerd Font icons: their ink is about two cells
-- wide, so they show how ng fits glyphs into cells.
local function files_float()
  local lines = {
    icon(0xf07b) .. ' cmd/',
    icon(0xf07b) .. ' internal/',
    icon(0xe627) .. ' go.mod',
    icon(0xe620) .. ' demo.lua',
    icon(0xe609) .. ' README.md',
    icon(0xe702) .. ' .gitignore',
  }
  local buf = vim.api.nvim_create_buf(false, true)
  vim.api.nvim_buf_set_lines(buf, 0, -1, false, lines)
  local function place()
    return { relative = 'editor', anchor = 'NE', row = 1, col = vim.o.columns - 2 }
  end
  local win = vim.api.nvim_open_win(
    buf,
    false,
    vim.tbl_extend('force', place(), {
      width = 20,
      height = #lines,
      border = 'rounded',
      title = ' ファイル ',
      title_pos = 'center',
    })
  )
  -- A tiling window manager may resize the window after startup.
  vim.api.nvim_create_autocmd('VimResized', {
    callback = function()
      vim.api.nvim_win_set_config(win, place())
    end,
  })
end

local scenes = {}

function scenes.edit()
  vim.cmd.edit(vim.fn.fnameescape(root .. '/cmd/ng/main.go'))
  vim.fn.search('^func run()')
  vim.cmd('normal! zt')
  local code = vim.api.nvim_get_current_win()
  vim.cmd('rightbelow vsplit ' .. vim.fn.fnameescape(dir .. '/sample.md'))
  -- Half of 100 columns: keep the Go code readable instead of wrapped.
  vim.wo[code][0].wrap = false
  vim.bo[vim.api.nvim_win_get_buf(code)].tabstop = 4
  vim.api.nvim_create_autocmd('VimResized', { command = 'wincmd =' })
  files_float()
end

function scenes.ime()
  vim.cmd.edit(vim.fn.fnameescape(dir .. '/sample.md'))
  vim.cmd('normal! G')
  vim.cmd('startinsert!')
end

function scenes.image()
  vim.cmd.edit(vim.fn.fnameescape(dir .. '/sample.md'))
  if not vim.ui.img then
    vim.notify('vim.ui.img needs Neovim 0.13 or later', vim.log.levels.ERROR)
    return
  end
  local png = vim.fn.readblob(root .. '/assets/icons/hicolor/256x256/apps/ng.png')
  -- nvim_ui_send() reaches ng only once ng has set stdout_tty, which it does
  -- after UIEnter (docs/images.md).
  vim.schedule(function()
    vim.wait(3000, function()
      for _, ui in ipairs(vim.api.nvim_list_uis()) do
        if ui.stdout_tty then
          return true
        end
      end
      return false
    end)
    if vim.ui.img._supported({ timeout = 3000 }) then
      vim.ui.img.set(png, { row = 3, col = vim.o.columns - 21, width = 16, height = 8 })
    else
      vim.notify('this UI does not support vim.ui.img', vim.log.levels.ERROR)
    end
  end)
end

vim.api.nvim_create_autocmd('VimEnter', {
  once = true,
  callback = function()
    local f = scenes[scene]
    if not f then
      local names = vim.tbl_keys(scenes)
      table.sort(names)
      vim.notify(
        ('unknown NG_DEMO=%s: use one of %s'):format(scene, table.concat(names, ', ')),
        vim.log.levels.ERROR
      )
      return
    end
    f()
  end,
})

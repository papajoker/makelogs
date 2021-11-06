#!/usr/bin/env python
# TODO ?
"""
use pkexec if sudo is in one command ?
"""
import console as mlog
from pathlib import Path
import subprocess
import gi
gi.require_version("Gtk", "3.0")
from gi.repository import Gtk, Gdk


'''
gui code from : https://python-gtk-3-tutorial.readthedocs.io/en/latest/treeview.html#filtering
'''

# with gui can't use current dir, use /tmp/ ?
log_filename = Path.home() / "logs.md"

# list of tuples for each command
store_command = list(mlog.store_commands())
print(store_command)


class COL:
    CAT, COMMAND, DESC, RUN, ID, CMDTORUN, BASH = list(range(7))


class TreeViewFilterWindow(Gtk.Window):
    def __init__(self):
        super().__init__(title="MakeLogs Gui")

        self.set_default_size(800, 600)
        self.set_position(Gtk.WindowPosition.CENTER_ALWAYS)
        self.set_border_width(10)
        '''
        icontheme = Gtk.IconTheme.get_default()
        print([i for i in icontheme.list_icons() if "log" in i])
        self.set_icon_name("help-about")
        '''
        self.set_icon_name("text-x-log")

        grid = Gtk.Grid()
        grid.set_column_homogeneous(True)
        grid.set_row_homogeneous(True)
        self.add(grid)

        # Creating the ListStore model
        self.store = Gtk.ListStore(str, str, str, bool, int, str, str)
        for cmd in store_command:
            self.store.append(list(cmd))
        self.current_filter_category = None

        # Creating the filter
        self.category_filter = self.store.filter_new()
        self.category_filter.set_visible_func(self.category_filter_func)

        self.treeview = Gtk.TreeView(model=self.category_filter)
        # self.treeView.set_activate_on_single_click(True)
        self.treeview.set_property('activate-on-single-click', True)
        self.treeview.connect("row-activated", self.on_row_activate)

        for i, column_title in enumerate(
            ["Category", "Name", "Description", "To run..."]
        ):
            renderer = Gtk.CellRendererText()
            column = Gtk.TreeViewColumn(column_title, renderer, text=i)
            # column.set_sort_order(Gtk.SortType.DESCENDING)
            # column.set_sort_column_id(i)
            column.set_resizable(True)
            if i == COL.RUN:
                # column.set_visible(False)
                column.set_resizable(False)  # not possible with last :(
                column.set_max_width(40)
                column.set_fixed_width(40)
                column.set_cell_data_func(
                    renderer, self.treeview_cell_app_data_function, None)
            self.treeview.append_column(column)

        self.filters = list({f[0] for f in store_command})
        self.filters.sort()
        print(self.filters)
        self.filters.insert(0, "All")
        self.filters.insert(0, "To run...")

        # creating btns
        buttons = list()
        for item in self.filters:
            button = Gtk.Button(label=item)
            buttons.append(button)
            button.connect("clicked", self.on_category_btn)

        # setting up the layout
        scrollable_treelist = Gtk.ScrolledWindow()
        scrollable_treelist.set_vexpand(True)
        HEIGHT = 12
        WIDTH = 9
        grid.attach(scrollable_treelist, 0, 0, WIDTH, HEIGHT-3)  # 9w 12 h

        box = Gtk.Box(orientation=Gtk.Orientation.HORIZONTAL,
                      homogeneous=True, spacing=4)
        for i, button in enumerate(buttons[1:]):
            # box.add(button)
            box.pack_start(button, True, True, 10)
        grid.attach(box, 0, HEIGHT-2, WIDTH, 1)
        grid.attach(buttons[0], WIDTH-1, HEIGHT-0, 1, 1)
        self.buttonr = buttons[0]
        self.buttonr.set_sensitive(False)

        box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=10)
        box.set_homogeneous(False)
        self.label = Gtk.Label(label="")
        self.label.set_max_width_chars(255)
        box.pack_start(self.label, True, True, 0)
        grid.attach(box, 0, HEIGHT-1, WIDTH, 1)

        box = Gtk.Box(orientation=Gtk.Orientation.VERTICAL, spacing=0)
        self.button = Gtk.Button(
            label="RUN", image=Gtk.Image(stock=Gtk.STOCK_MEDIA_RECORD))
        self.button.set_property('margin_top', 10)
        self.button.connect("clicked", self.on_run_btn)
        self.button.set_sensitive(False)

        box.add(self.button)
        grid.attach(box, WIDTH/3, HEIGHT, WIDTH/3, 1)

        scrollable_treelist.add(self.treeview)

        self.show_all()

    def on_row_activate(self, treeview, path, column):
        # path is false after filter !
        model, iter = treeview.get_selection().get_selected()
        id = model.get_value(iter, COL.ID)
        self.store[id][COL.RUN] = not self.store[id][COL.RUN]
        r = {True for e in self.store if e[COL.RUN]}
        self.button.set_sensitive(bool(r))
        self.buttonr.set_sensitive(bool(r))
        self.label.set_text("")
        if self.store[id][COL.RUN]:
            self.label.set_markup(f"<small>{self.store[id][COL.BASH]}</small>")

    def category_filter_func(self, model, iter, data):
        """Tests if the language in the row is the one in the filter"""
        # self.filters
        if (
            self.current_filter_category is None
            or self.current_filter_category == self.filters[1]  # All
        ):
            return True
        elif self.current_filter_category == self.filters[0]:   # items to run
            return model[iter][COL.RUN]
        return model[iter][COL.CAT] == self.current_filter_category

    def on_run_btn(self, widget):
        """ create logs """
        self.label.set_text("")
        ret = [e[COL.CMDTORUN] for e in self.store if e[COL.RUN]]
        if not ret:
            return
        print(ret)

        watch_cursor = Gdk.Cursor(Gdk.CursorType.WATCH)
        self.treeview.get_window().set_cursor(watch_cursor)
        try:

            # use sudo ?
            sudo = [c[COL.COMMAND] for c in self.store if c[COL.RUN] and "sudo" in c[COL.BASH]]
            if sudo:
                print("\n\n", sudo)
                # TODO how to ?
                print("call with pkexec... or gui wait sudo command")
                cmd = f"pkexec {Path(__file__).parent.parent}/makelogs -r {' '.join(ret)}"
                print(cmd)
                subprocess.run(cmd, shell=True)
                # FIXME
                # pass logfilename in script makelogs as params
                # now : log_filename is tmp/makelogs :
                log_filename = Path(mlog.configdir).parent  / "logs.md"
            else:
                log_filename = Path.home() / "logs.md"
                mlog.main({
                    "caption": "My logs",
                    "actions": list(mlog.search_in_params(ret))
                },
                    log_filename)
        finally:
            self.treeview.get_window().set_cursor(None)
        dialog = Gtk.MessageDialog(
            transient_for=self,
            flags=0,
            message_type=Gtk.MessageType.INFO,
            buttons=Gtk.ButtonsType.CLOSE,
            text="log generated",
        )
        dialog.format_secondary_text(
            f"{log_filename}"
        )
        dialog.run()
        dialog.destroy()

        if Path("/usr/bin/qdbus").exists() and Path("/usr/bin/kate").exists():
            subprocess.run(f'qdbus org.kde.klauncher5 /KLauncher exec_blind "/usr/bin/kate" "{log_filename}"', shell=True)
        else:
            subprocess.run(f'xdg-open "{log_filename}" &', shell=True)

    def on_category_btn(self, widget):
        """Called on any of the button clicks"""
        self.current_filter_category = widget.get_label()
        self.category_filter.refilter()
        self.set_title(f"MakeLogs Gui - {self.current_filter_category}")
        self.label.set_text("")

    def treeview_cell_app_data_function(self, column: Gtk.TreeViewColumn, renderer_cell: Gtk.CellRenderer, model: Gtk.TreeModel, iter_a: Gtk.TreeIter, user_data):
        """display: column "selected" change font if ok"""
        # model.get(iter_a, CATS.RUN) return tuple(x,)
        if model.get(iter_a, COL.RUN)[0]:
            renderer_cell.props.weight = 500
            renderer_cell.set_property('text', "OK")
        else:
            renderer_cell.props.weight = 100
            renderer_cell.set_property('text', "-")


if __name__ == "gtkgui" or __name__ == "__main__":

    win = TreeViewFilterWindow()
    win.connect("destroy", Gtk.main_quit)
    win.show_all()
    Gtk.main()

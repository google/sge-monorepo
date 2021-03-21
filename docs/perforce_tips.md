# Perforce

## Tips

*   To show the dates and time in your **local time**, you can go to
    `Edit->Preferences->Display->Show date and time as: Local time`.

*   If you are using the command-line p4 tool and would like to change the default editor to VSCode,
    use this:

```
set P4EDITOR="%USERPROFILE%\AppData\Local\Programs\Microsoft VS Code\Code.exe -w"
```

*   If you are using the command-line p4 tool, you can use the "-N" flag to find out how many
    files/bytes will get transferred:

```
p4 sync -N ...
```

*   To improve the readability of the Log section at the bottom of the P4V window, right-click on it
    and press view timestamp.

*   To more easily see the files you've changed in your workspace:
    `Edit->Preferences->Files And History ->Use a distinct icon for modified files`

*   Perforce has a *Reconcile Offline Work...* feature that you can access by right-clicking on a
    folder in the depot or workspace views. This will process all your local files in that folder
    and tell you which files you have locally which aren't in the depot and vice versa.

    *   Note that this will process *all* the files in that folder, this can take quite a while
        (on the order of hours) for larger projects, in particular projects later in their
        development cycle. This shouldn't be part of your normal workflow, but can be helpful in
        certain situations, such as when adding a large set of new files that might include
        generated files.

    *   To make this useful, you will want to make use of a p4ignore file. The project you are
        working on will likely have one checked in to one of its top level folders. Find the
        location of that file on your local disk and then set the P4IGNORE environment variable to
        that file path. (You will have to restart P4V for it to take effect.)

## Workflow Tips

*   **Create a DO NOT SUBMIT change list**

    Every once in a while, you’ll need to locally edit a file that you don’t want to check in. CLs
    are sorted in numerical order in your client window, so creating this first and never deleting
    it means it will always be on top.

    TIP: Try not to keep the files checked out for long. You can inadvertently end up needing to
    make a change you do want to submit and forgetting its in your DNS CL, making it easy to miss
    when submitting for review or check-in.

*   **Shelve CLs Often** - Daily at least

    Shelves back up your files on the P4 server. This is useful in case of HDD failure (it happens!)

    TIP: Be sure to uncheck the box that I believe defaults to deleting local files when the shelf
    is created. It should remember that change after the 1st time.

*   **Create a CL when you start work on a feature**

    Working out of default is OK, but the discipline of creating the new CL for the feature and
    writing the skeleton of the CL description (tags, overall purpose of the CL, etc) is useful to
    focus work. It also makes it trivial to task-swap mid-feature: move any files in default to your
    in-progress CL, shelve, revert if needed, and start work on the new task.

    TIP: It requires taking time up front and maintaining the CL as you go rather than at the end.
    This runs counter to some people’s optimal working flow, so may not be good for everyone.

*   **Try not to have too many CLs in flight at once**

    Lots of CLs in progress means lots of task swapping, which is hard. It also means you’re more
    likely to end up having more than one CL need to edit a particular file. Shelves help with that,
    but can be a little annoying to negotiate. Better to minimize the number of active CLs at any
    one time.

    TIP: It can be tough to wrap up particular features sometimes - you may be blocked by someone or
    something and have to hang tight to your CL in the meantime.

*   **Refresh P4 before submitting for review / submitting the CL**

    Especially if you have the P4 plugins for Unreal / Unity / VisDev, they can check out files that
    the P4V GUI is unaware of. Those plugins will likely check files out to the default CL, and if
    you don’t refresh the GUI, you may miss that they are checked out in the default CL - thus
    missing the files when submitting for review or checking in.

    TIP: Literally none - just hit refresh!

*   **Write good CL descriptions!**

    Google / SG&E CL description requirements cover a lot of this, but it can’t be stressed enough
    how useful it has been to have a thorough CL description to look at when having to dive back
    through the history of a file.

    TIP: It takes some effort up front, and you won’t see the benefit for a while. Usually this only
    starts to pay off later in the project.

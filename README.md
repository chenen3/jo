The code implements a text editor that supports the following features:
- syntax highlighting
- search
- tabs
- go to any file or line
- code completion
- split view
- undo and redo

The implementation depends on [tcell](https://github.com/gdamore/tcell),
so it works in the terminal, but draws the UI from scratch 
and manages focus, click, hover and popup itself.

Here are the screenshots:

<img width="777" alt="search" src="https://github.com/user-attachments/assets/dc6c1232-b351-4877-a2e6-550bf68f391e">
<img width="777" alt="completion" src="https://github.com/user-attachments/assets/af7b9ecc-8260-48e4-a9be-c4e5104a5b6e">

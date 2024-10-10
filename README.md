<h1>TXSS - Path Based Special Character Reflection Checker</h1>

<b>Install directly</b> <br>
<code>go install -v github.com/SMHTahsin33/txss@latest</code>

<b>Install from the source:</b> <br>
<code>go build -o txss main.go</code>

<h2>Usage:</h2>
<img width="1151" alt="Screenshot 2024-10-11 at 4 00 36 AM" src="https://github.com/user-attachments/assets/a65ede61-c982-4f65-88ed-1991042a28fa">
<img width="948" alt="Screenshot 2024-10-11 at 4 01 58 AM" src="https://github.com/user-attachments/assets/59a70b22-ae3d-4136-89dd-6a562bf3e71a">
<h2>Information</h2>
● It will check only for path based reflections.<br>
● It will ignore the query part (if there are any)<br>
● If there is any redirection initially, it will follow and test the final URL<br>
● Can access a little bit of debug info with <code>-debug</code> flag<br>
● Can set thread with <code>-t</code> flag<br>
<br>
If you want to contribute and make it better just make a pull request!
<br>
<br>
-SMHTahsin33

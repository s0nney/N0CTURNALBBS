<h1 align="center">
N0CTURNALBBS
</h1>
<p align="center">
<img src="https://github.com/user-attachments/assets/a8b3f7dd-219c-4e7d-9fa8-4dc6173ce2cf"/>
 <br>A textboard engine built on the stuff of nightmares and minimalism in mind.
</p>

```
# IDEAL TECH STACK
- Go 1.24.0+
- PSQL
- NGINX
```

```
# SETUP 
- Configure all files within /config accordingly. 
- Character settings for threads and replies can also be tinkered with in /internal/handlers/board.go.
- Salt generation for tripcodes must also be configured in models/post.go.
- Copy the exact table structure in db.sql.
- Disallow directory viewing for /templates when in production
```
<p align="center">
You shouldn't experience any hiccups afterwards...fingers crossed.
</p>
<p align="center"><b><i>N0CTURNALBBS v1.1</i></b></p>

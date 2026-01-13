import httpcloak as hc

session = hc.Session(preset=hc.Preset.CHROME_143, http_version="h2")
req = session.get("https://www.walmart.com/ip/Sabrina-Carpenter-Cherry-Pop-EDP-30ml-1oz/5492571361")
print(req.status_code)
with open("stuff.html", "w", encoding="utf-8") as f:
    f.write(req.text)
print(req.url)
print(req.final_url)
import json
import urllib.request
import urllib.error
import glob
import os

URL = "http://localhost:8080/run"

def test_file(file_path):
    print(f"Testing {file_path}...")
    with open(file_path, "r") as f:
        data = f.read()
    
    req = urllib.request.Request(URL, data=data.encode('utf-8'), headers={'Content-Type': 'application/json'})
    try:
        with urllib.request.urlopen(req) as response:
            res_data = response.read().decode('utf-8')
            res_json = json.loads(res_data)
            print(f"Result: {res_json.get('status')}")
            # print(f"Response: {res_json}")
    except urllib.error.HTTPError as e:
        print(f"HTTP Error {e.code}: {e.read().decode('utf-8')}")
    except Exception as e:
        print(f"Error: {e}")

def main():
    files = glob.glob("testdata/*.json")
    for f in sorted(files):
        if any(lang in f for lang in ["java", "ruby", "zig"]):
            test_file(f)

if __name__ == "__main__":
    main()

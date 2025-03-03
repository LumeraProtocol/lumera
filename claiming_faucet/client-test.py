#!/usr/bin/env python3
import requests
import json

if __name__ == "__main__":
    # Update these values with your actual data
    SERVER_URL = "http://localhost:8080"  # Faucet server URL

    OLD_ADDRESS = "PtVvg5DrnqZpXSWKaATBCqsA3Cssnwb2REv"
    OLD_PUB_KEY = "038c8f7753753b81531338a7d12a583051054aadd823624a599bad2adb0f68a19b"
    NEW_ADDRESS = "lumera13n5463segh5egmndmeyk398frlhpy8usyc5em5"
    SIGNATURE = "205d1ddcfe96a301cddbcd1189b687f7d74245eb8b4d9c75e392081211326aec930b5fa0ae9165e7da81ea0f1e2d6e03fed4e9d5a1fd04a17c2558e6aa66802041"

    url = f"{SERVER_URL}/api/getfeeforclaiming"
    payload = {
        "old_address": OLD_ADDRESS,
        "old_pub_key": OLD_PUB_KEY,
        "new_address": NEW_ADDRESS,
        "signature": SIGNATURE
    }

    try:
        response = requests.post(url, json=payload)
        # 400 Bad Request:
        #   "Account already exists"
        #   "Claim record not found"
        #   "Claim already processed"
        #   "Mismatch in claim address"
        #   "Mismatch in claim public key"
        #   "Invalid signature"
        #   "Mismatch in reconstructed address"
        #   "Invalid signature"
        response.raise_for_status()
        print("Response received:")
        print(json.dumps(response.json(), indent=4))
    except requests.exceptions.RequestException as e:
        print(f"An error occurred: {e}")

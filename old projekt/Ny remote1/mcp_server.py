from flask import Flask, request, jsonify
from flask_cors import CORS
import redis
import json

app = Flask(__name__)
CORS(app)

# Connect to Redis
redis_client = redis.Redis(host='localhost', port=6379, db=0, decode_responses=True)

@app.route('/memory', methods=['POST'])
def save_memory():
    data = request.json
    key = data.get('key')
    value = data.get('value')
    if not key or not value:
        return jsonify({'error': 'Key and value are required'}), 400
    redis_client.set(key, json.dumps(value))
    return jsonify({'message': 'Memory saved successfully'})

@app.route('/memory/<key>', methods=['GET'])
def get_memory(key):
    value = redis_client.get(key)
    if not value:
        return jsonify({'error': 'Key not found'}), 404
    return jsonify({'key': key, 'value': json.loads(value)})

if __name__ == '__main__':
    app.run(host='0.0.0.0', port=5000)

#!/usr/bin/env python3
# -*- coding: utf-8 -*-

"""
OCR验证码识别服务
使用ddddocr进行验证码识别，提供HTTP API接口
"""

from flask import Flask, request, jsonify
import ddddocr
import base64
import logging
import os
from datetime import datetime
import traceback

# 配置日志
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(name)s - %(levelname)s - %(message)s'
)
logger = logging.getLogger(__name__)

app = Flask(__name__)

# 初始化OCR引擎
ocr = ddddocr.DdddOcr(show_ad=False)  # 关闭广告
logger.info("OCR引擎初始化成功")

@app.route('/health', methods=['GET'])
def health_check():
    """健康检查接口"""
    return jsonify({
        'status': 'healthy',
        'service': 'OCR Service',
        'timestamp': datetime.now().isoformat()
    })

@app.route('/recognize', methods=['POST'])
def recognize():
    """
    识别验证码
    请求格式: {"image": "base64编码的图片"}
    响应格式: {"success": true, "code": "识别结果", "confidence": 0.95}
    """
    try:
        data = request.json
        if not data or 'image' not in data:
            return jsonify({
                'success': False,
                'error': 'Missing image data'
            }), 400
        
        # 解码base64图片
        image_base64 = data['image']
        # 移除可能的data:image前缀
        if ',' in image_base64:
            image_base64 = image_base64.split(',')[1]
        
        image_bytes = base64.b64decode(image_base64)
        
        # 识别验证码
        result = ocr.classification(image_bytes)
        
        logger.info(f"验证码识别成功: {result}")
        
        return jsonify({
            'success': True,
            'code': result,
            'confidence': 0.95  # ddddocr不提供置信度，这里给一个默认值
        })
        
    except Exception as e:
        logger.error(f"验证码识别失败: {str(e)}")
        logger.error(traceback.format_exc())
        return jsonify({
            'success': False,
            'error': str(e)
        }), 500

@app.route('/recognize/sliding', methods=['POST'])
def recognize_sliding():
    """
    识别滑动验证码
    请求格式: {"background": "背景图base64", "slider": "滑块图base64"}
    响应格式: {"success": true, "distance": 123}
    """
    try:
        data = request.json
        if not data or 'background' not in data:
            return jsonify({
                'success': False,
                'error': 'Missing background image'
            }), 400
        
        background_base64 = data['background']
        if ',' in background_base64:
            background_base64 = background_base64.split(',')[1]
        background_bytes = base64.b64decode(background_base64)
        
        # 使用ddddocr的滑块识别功能
        slider_ocr = ddddocr.DdddOcr(det=False, ocr=False, show_ad=False)
        
        if 'slider' in data:
            slider_base64 = data['slider']
            if ',' in slider_base64:
                slider_base64 = slider_base64.split(',')[1]
            slider_bytes = base64.b64decode(slider_base64)
            
            # 识别滑块缺口位置
            res = slider_ocr.slide_match(slider_bytes, background_bytes, simple_target=True)
            distance = res['target'][0]  # x坐标
        else:
            # 只有背景图，识别缺口位置
            res = slider_ocr.slide_comparison(background_bytes)
            distance = res['target'][0]
        
        logger.info(f"滑块验证码识别成功: 距离={distance}")
        
        return jsonify({
            'success': True,
            'distance': distance
        })
        
    except Exception as e:
        logger.error(f"滑块验证码识别失败: {str(e)}")
        logger.error(traceback.format_exc())
        return jsonify({
            'success': False,
            'error': str(e)
        }), 500

if __name__ == '__main__':
    port = int(os.environ.get('OCR_PORT', 8888))
    host = os.environ.get('OCR_HOST', '0.0.0.0')
    
    logger.info(f"OCR服务启动在 {host}:{port}")
    app.run(host=host, port=port, debug=False)
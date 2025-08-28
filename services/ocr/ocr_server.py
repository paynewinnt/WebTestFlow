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
import time
from datetime import datetime
import traceback
import numpy as np
from PIL import ImageFilter

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

def find_gap_with_edge_detection(img, width, height):
    """使用改进的边缘检测寻找缺口位置"""
    try:
        # 转换为灰度图
        gray = img.convert('L')
        
        # 使用边缘检测
        edges = gray.filter(ImageFilter.FIND_EDGES)
        
        # 转换为numpy数组分析
        edge_array = np.array(edges)
        
        # 分析整个图像宽度，而不只是右半部分
        # 计算每列的边缘强度
        col_edges = np.sum(edge_array, axis=0)
        
        # 使用滑动窗口寻找缺口特征
        window_size = max(10, width // 20)  # 动态窗口大小
        best_gap_x = width // 4  # 默认位置
        best_score = float('inf')
        
        # 在有效范围内搜索（避免图像边缘）
        search_start = width // 10
        search_end = width - width // 10
        
        for x in range(search_start, search_end - window_size):
            # 计算窗口内的边缘强度变化
            window = col_edges[x:x+window_size]
            
            # 寻找边缘强度突然下降然后上升的位置（缺口特征）
            if len(window) > 2:
                # 计算边缘强度的方差（缺口处应该有明显变化）
                variance = np.var(window)
                # 计算平均强度（缺口处应该较低）
                mean_intensity = np.mean(window)
                
                # 综合得分：方差大且平均强度低的位置更可能是缺口
                score = mean_intensity - variance * 0.5
                
                if score < best_score:
                    best_score = score
                    best_gap_x = x + window_size // 2
        
        logger.info(f"改进边缘检测找到疑似缺口位置: {best_gap_x}px (得分: {best_score:.2f})")
        return int(best_gap_x)  # 转换为标准Python int
        
    except Exception as e:
        logger.error(f"边缘检测失败: {e}")
        raise e

def find_gap_with_color_analysis(img, width, height):
    """使用改进的颜色差异分析寻找缺口位置"""
    try:
        img_array = np.array(img.convert('RGB'))
        
        # 计算图像的平均颜色
        avg_color = np.mean(img_array, axis=(0, 1))
        logger.info(f"图像平均颜色: RGB{tuple(avg_color.astype(int))}")
        
        # 寻找与平均颜色差异较大的区域
        color_diff = np.sqrt(np.sum((img_array - avg_color) ** 2, axis=2))
        
        # 分析整个图像宽度
        col_diff = np.mean(color_diff, axis=0)
        
        # 使用改进的搜索策略
        window_size = max(8, width // 25)  # 更小的窗口，更精确
        best_gap_x = width // 4  # 默认位置
        best_score = -1
        
        # 在有效范围内搜索（避免边缘噪声）
        search_start = width // 8
        search_end = width - width // 8
        
        for x in range(search_start, search_end - window_size):
            window = col_diff[x:x+window_size]
            
            # 寻找颜色差异的特殊模式
            if len(window) > 2:
                # 计算窗口内的最大差异
                max_diff = np.max(window)
                # 计算差异的标准差（缺口处应该有突变）
                std_diff = np.std(window)
                
                # 综合得分：高差异且有变化的位置更可能是缺口
                score = max_diff + std_diff * 0.3
                
                if score > best_score:
                    best_score = score
                    best_gap_x = x + window_size // 2
        
        logger.info(f"改进颜色分析找到疑似缺口位置: {best_gap_x}px (得分: {best_score:.2f})")
        return int(best_gap_x)  # 转换为标准Python int
        
    except Exception as e:
        logger.error(f"颜色分析失败: {e}")
        raise e

def find_precise_gap_boundary(img_array, width, height, edge_image):
    """精确的拼图块边界匹配算法"""
    try:
        import cv2
        
        # 1. 创建更精确的边缘图像
        gray = cv2.cvtColor(img_array, cv2.COLOR_RGB2GRAY)
        
        # 2. 使用高斯模糊减少噪声
        blurred = cv2.GaussianBlur(gray, (5, 5), 0)
        
        # 3. 使用自适应阈值增强拼图边界
        adaptive_thresh = cv2.adaptiveThreshold(blurred, 255, cv2.ADAPTIVE_THRESH_GAUSSIAN_C, cv2.THRESH_BINARY, 11, 2)
        
        # 4. 形态学操作，清理边界
        kernel = np.ones((3,3), np.uint8)
        cleaned = cv2.morphologyEx(adaptive_thresh, cv2.MORPH_CLOSE, kernel)
        
        # 5. 寻找垂直边界线
        # 分析每列的像素变化，寻找拼图块的左边界
        col_changes = []
        for x in range(width):
            col = cleaned[:, x]
            # 计算该列的变化点数量（黑白转换）
            changes = 0
            for y in range(1, height):
                if col[y] != col[y-1]:
                    changes += 1
            col_changes.append(changes)
        
        # 6. 寻找变化最明显的区域（拼图块边界）
        max_changes = max(col_changes) if col_changes else 0
        if max_changes > 4:  # 至少有一定数量的变化
            # 寻找变化数量的峰值
            peaks = []
            for x in range(2, width-2):
                if (col_changes[x] > col_changes[x-1] and 
                    col_changes[x] > col_changes[x+1] and
                    col_changes[x] > max_changes * 0.5):  # 至少是最大值的50%
                    peaks.append((x, col_changes[x]))
            
            if peaks:
                # 选择变化最明显的位置
                best_peak = max(peaks, key=lambda p: p[1])
                precise_x = best_peak[0]
                
                # 进一步微调：在峰值附近寻找最精确的边界
                search_range = 5
                start_x = max(0, precise_x - search_range)
                end_x = min(width, precise_x + search_range)
                
                best_boundary_x = precise_x
                max_gradient = 0
                
                for x in range(start_x, end_x):
                    if x > 0 and x < width - 1:
                        # 计算水平梯度
                        left_col = np.mean(gray[:, x-1])
                        right_col = np.mean(gray[:, x+1])
                        gradient = abs(right_col - left_col)
                        
                        if gradient > max_gradient:
                            max_gradient = gradient
                            best_boundary_x = x
                
                logger.info(f"精确边界检测: 峰值{precise_x}px, 微调后{best_boundary_x}px, 梯度{max_gradient:.1f}")
                return best_boundary_x
                
        return None
        
    except Exception as e:
        logger.error(f"精确边界匹配失败: {e}")
        return None

def find_gap_with_puzzle_template_matching(img, width, height):
    """使用改进的拼图缺口识别算法寻找真正的缺口位置"""
    logger.info("使用改进的拼图缺口识别算法寻找缺口位置")
    
    try:
        import cv2
        
        # 转换为OpenCV格式
        img_array = np.array(img)
        gray = cv2.cvtColor(img_array, cv2.COLOR_RGB2GRAY)
        
        # 增强对比度，但保持拼图块和缺口的区别
        clahe = cv2.createCLAHE(clipLimit=2.0, tileGridSize=(8,8))
        enhanced = clahe.apply(gray)
        
        # 核心修复：寻找拼图块位置，然后计算缺口应该在的位置
        # 1. 先找到拼图块本身
        edges_canny = cv2.Canny(enhanced, 30, 90, apertureSize=3)
        
        # 2. 寻找拼图块的轮廓
        contours, _ = cv2.findContours(edges_canny, cv2.RETR_EXTERNAL, cv2.CHAIN_APPROX_SIMPLE)
        
        # 3. 找到最像拼图块的轮廓（通常是最大的非矩形轮廓）
        puzzle_piece_x = None
        max_area = 0
        
        for contour in contours:
            area = cv2.contourArea(contour)
            if area > 200:  # 足够大的轮廓
                # 计算轮廓的边界框
                x, y, w, h = cv2.boundingRect(contour)
                
                # 检查是否像拼图块（宽高比合理，面积适中）
                aspect_ratio = w / h if h > 0 else 0
                if 0.5 < aspect_ratio < 2.0 and area > max_area:
                    max_area = area
                    puzzle_piece_x = x + w // 2  # 拼图块中心位置
        
        if puzzle_piece_x is not None:
            logger.info(f"检测到拼图块中心位置: {puzzle_piece_x}px")
            
            # 4. 根据拼图块位置推断缺口位置
            # 改进推断逻辑：不使用简单镜像，而是基于合理的位置分布
            if puzzle_piece_x < width * 0.3:
                # 拼图块在左侧，缺口可能在中右侧
                gap_x = int(width * 0.6 + (width * 0.3 - puzzle_piece_x) * 0.8)
                logger.info(f"拼图块在左侧({puzzle_piece_x}px)，推断缺口在中右侧({gap_x}px)")
            elif puzzle_piece_x > width * 0.7:
                # 拼图块在右侧，缺口可能在中左侧
                gap_x = int(width * 0.4 - (puzzle_piece_x - width * 0.7) * 0.8)
                logger.info(f"拼图块在右侧({puzzle_piece_x}px)，推断缺口在中左侧({gap_x}px)")
            else:
                # 拼图块在中间，缺口可能在两侧
                if puzzle_piece_x < width // 2:
                    gap_x = int(width * 0.75)  # 右侧
                    logger.info(f"拼图块在中左({puzzle_piece_x}px)，推断缺口在右侧({gap_x}px)")
                else:
                    gap_x = int(width * 0.25)  # 左侧
                    logger.info(f"拼图块在中右({puzzle_piece_x}px)，推断缺口在左侧({gap_x}px)")
            
            # 5. 验证推断的缺口位置是否合理
            if 20 < gap_x < width - 20:
                logger.info(f"基于拼图块位置推断的缺口位置: {gap_x}px")
                return int(gap_x)
        
        # 备用方法：直接寻找缺口特征
        logger.info("未找到明显拼图块，使用缺口直接识别方法")
        
        # 寻找拼图缺口的特征形状
        # 缺口特征：相对平坦的区域，边缘密度较低但有一定变化
        
        # 分析每列的边缘特征
        col_edges = np.sum(edges_canny, axis=0)
        
        # 寻找可能的缺口区域 - 寻找边缘密度较低的平坦区域
        candidates = []
        window_size = max(20, width // 20)  # 稍大窗口以检测缺口区域
        
        for x in range(window_size, width - window_size):
            # 计算窗口内的边缘特征
            window_edges = col_edges[x-window_size:x+window_size]
            
            # 特征1：边缘密度（缺口应该边缘密度较低）
            total_density = np.sum(window_edges)
            avg_density = total_density / (window_size * 2)
            
            # 特征2：边缘平坦度（缺口内部相对平坦）
            edge_variance = np.var(window_edges)
            
            # 特征3：左右边界检测（缺口两侧应该有明显边界）
            left_boundary = np.mean(col_edges[max(0, x-window_size-5):x-window_size+5])
            right_boundary = np.mean(col_edges[x+window_size-5:min(width, x+window_size+5)])
            boundary_strength = (left_boundary + right_boundary) / 2
            
            # 特征4：与周围的对比度
            surrounding_density = (np.sum(col_edges[max(0, x-window_size*2):x-window_size]) + 
                                  np.sum(col_edges[x+window_size:min(width, x+window_size*2)])) / (window_size * 2)
            
            # 重新设计评分系统：专门识别缺口特征
            score = 0
            
            # 特征1: 缺口应该边缘密度较低
            if avg_density < 30:  # 低边缘密度
                density_score = (30 - avg_density) * 2  # 越低分数越高
                score += min(density_score, 50)
            elif avg_density < 60:  # 中等密度也可以接受
                density_score = (60 - avg_density) * 1
                score += min(density_score, 30)
            else:
                score -= 20  # 高密度区域不太可能是缺口
            
            # 特征2: 缺口应该相对平坦（低变化度）
            if edge_variance < 500:  # 平坦区域
                flatness_score = (500 - edge_variance) * 0.1
                score += min(flatness_score, 40)
            else:
                score -= 10  # 变化太大不是缺口
            
            # 特征3: 缺口两侧应该有边界
            if boundary_strength > 20:  # 两侧有明显边界
                boundary_score = min(boundary_strength * 0.5, 30)
                score += boundary_score
            
            # 特征4: 缺口与周围的对比
            if surrounding_density > avg_density * 1.5:  # 周围比缺口密度高
                contrast_score = min((surrounding_density - avg_density) * 0.3, 25)
                score += contrast_score
            
            # 特征5: 位置合理性（缺口通常不在最边缘）
            if width * 0.15 < x < width * 0.85:
                position_score = 20
                score += position_score
            elif width * 0.05 < x < width * 0.95:
                score += 10
            else:
                score -= 30  # 边缘位置不太可能是缺口
            
            candidates.append((x, score, avg_density, edge_variance, boundary_strength))
        
        # 调试：记录所有候选位置
        logger.info(f"候选位置总数: {len(candidates)}")
        if len(candidates) > 0:
            # 显示前5个最高分候选
            top_candidates = sorted(candidates, key=lambda item: item[1], reverse=True)[:5]
            for i, (x, score, density, variance, boundary) in enumerate(top_candidates):
                logger.info(f"  候选{i+1}: {x}px, 得分:{score:.2f}, 密度:{density:.1f}, 平坦度:{variance:.1f}, 边界:{boundary:.1f}")
        
        # 按得分排序，找到最佳候选位置
        candidates.sort(key=lambda item: item[1], reverse=True)
        
        if candidates:
            best_x = candidates[0][0]
            best_score = candidates[0][1]
            best_density = candidates[0][2]
            best_variance = candidates[0][3]
            best_boundary = candidates[0][4]
            
            logger.info(f"缺口识别算法找到疑似缺口位置: {best_x}px")
            logger.info(f"  得分: {best_score:.2f}, 密度: {best_density:.1f}, 平坦度: {best_variance:.1f}, 边界强度: {best_boundary:.1f}")
            
            # 简化验证 - 基本合理性检查
            if best_score > 30:  # 分数足够高
                logger.info(f"缺口验证通过，确认缺口位置: {best_x}px (得分: {best_score:.2f})")
                return int(best_x)
            else:
                logger.info(f"缺口候选位置分数较低({best_score:.2f})，继续尝试其他方法")
        
        # 如果所有方法都失败，返回保守估计
        logger.warning("所有缺口识别方法都失败，使用默认位置")
        return int(width * 0.3)  # 返回左侧30%位置作为默认估计
        
    except Exception as e:
        logger.error(f"拼图缺口识别失败: {e}")
        return int(width * 0.3)  # 返回保守估计

def find_gap_with_contour_fallback(img_array, width, height):
    """轮廓分析的备用方法"""
    try:
        from scipy import ndimage
        
        # 应用高斯模糊减少噪声
        blurred = ndimage.gaussian_filter(img_array, sigma=1.5)
        
        # 更精确的边缘检测：使用拉普拉斯算子
        laplacian = ndimage.laplace(blurred)
        laplacian_abs = np.abs(laplacian)
        
        # 分析每列的边缘密度变化
        col_edges = np.sum(laplacian_abs, axis=0)
        
        # 使用滑动窗口找到边缘密度最低的区域
        window_size = max(15, width // 20)
        min_density = float('inf')
        best_gap_x = width // 4
        
        search_start = width // 8
        search_end = width - width // 8
        
        for x in range(search_start, search_end - window_size):
            window_density = np.mean(col_edges[x:x+window_size])
            
            # 边缘密度低的地方更可能是缺口
            if window_density < min_density:
                min_density = window_density
                best_gap_x = x + window_size // 2
        
        logger.info(f"轮廓备用方法找到疑似缺口位置: {best_gap_x}px (最低密度: {min_density:.2f})")
        return int(best_gap_x)
        
    except Exception as e:
        logger.warning(f"轮廓备用方法失败: {e}")
        return int(width // 4)

@app.route('/health', methods=['GET'])
def health_check():
    """健康检查接口"""
    return jsonify({
        'status': 'healthy',
        'service': 'OCR Service',
        'timestamp': datetime.now().isoformat()
    })

def preprocess_captcha_image(image_bytes):
    """预处理验证码图片以提高识别率"""
    try:
        from PIL import Image, ImageEnhance, ImageOps, ImageFilter
        import io
        
        # 打开图片
        img = Image.open(io.BytesIO(image_bytes))
        logger.info(f"原始图片信息: 格式={img.format}, 模式={img.mode}, 尺寸={img.size}")
        
        # 保存原始图片用于调试
        debug_path = f"tmp/original_captcha_{int(time.time())}.png"
        img.save(debug_path, format='PNG')
        logger.info(f"原始图片已保存: {debug_path}")
        
        # 1. 转换为RGB模式（去除透明度）
        if img.mode in ('RGBA', 'LA'):
            # 创建白色背景
            background = Image.new('RGB', img.size, (255, 255, 255))
            if img.mode == 'RGBA':
                background.paste(img, mask=img.split()[-1])  # 使用alpha通道作为mask
            else:
                background.paste(img, mask=img.split()[-1])
            img = background
        elif img.mode != 'RGB':
            img = img.convert('RGB')
        
        # 2. 放大图片（如果太小的话）
        width, height = img.size
        if width < 100 or height < 40:
            # 放大到至少100x40
            scale_factor = max(100/width, 40/height, 2.0)  # 至少放大2倍
            new_width = int(width * scale_factor)
            new_height = int(height * scale_factor)
            img = img.resize((new_width, new_height), Image.Resampling.LANCZOS)
            logger.info(f"图片放大: {width}x{height} -> {new_width}x{new_height}")
        
        # 3. 增强对比度
        enhancer = ImageEnhance.Contrast(img)
        img = enhancer.enhance(1.5)  # 增强对比度
        
        # 4. 增强清晰度
        enhancer = ImageEnhance.Sharpness(img)
        img = enhancer.enhance(1.3)  # 增强清晰度
        
        # 5. 去噪声（轻微的模糊然后锐化）
        img = img.filter(ImageFilter.MedianFilter(size=3))
        
        # 6. 转换为灰度图进行二值化处理
        gray_img = ImageOps.grayscale(img)
        
        # 7. 自适应二值化
        import numpy as np
        
        # 转换为numpy数组
        img_array = np.array(gray_img)
        
        # 计算自适应阈值
        mean_brightness = np.mean(img_array)
        std_brightness = np.std(img_array)
        
        # 动态阈值：根据图片特征调整
        if std_brightness < 30:  # 低对比度图片
            threshold = mean_brightness
        else:  # 正常对比度图片
            threshold = mean_brightness - std_brightness * 0.3
        
        threshold = max(120, min(200, threshold))  # 限制阈值范围
        
        # 应用二值化
        binary_array = np.where(img_array > threshold, 255, 0).astype(np.uint8)
        binary_img = Image.fromarray(binary_array)
        
        # 8. 保存处理后的图片用于调试
        processed_path = f"tmp/processed_captcha_{int(time.time())}.png"
        binary_img.save(processed_path, format='PNG')
        logger.info(f"处理后图片已保存: {processed_path}")
        
        # 9. 转换回bytes
        output_buffer = io.BytesIO()
        binary_img.save(output_buffer, format='PNG')
        processed_bytes = output_buffer.getvalue()
        
        logger.info(f"图片预处理完成: {len(image_bytes)}字节 -> {len(processed_bytes)}字节")
        return processed_bytes
        
    except Exception as e:
        logger.error(f"图片预处理失败: {e}")
        # 预处理失败时返回原始图片
        return image_bytes

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
        logger.info(f"接收到验证码图片: {len(image_bytes)}字节")
        
        # 预处理图片
        processed_bytes = preprocess_captcha_image(image_bytes)
        
        # 使用原始图片和处理后的图片分别尝试识别
        results = []
        
        # 方法1：使用原始图片
        try:
            result1 = ocr.classification(image_bytes)
            if result1 and result1.strip():
                results.append(('原始图片', result1.strip()))
                logger.info(f"原始图片识别结果: '{result1}'")
            else:
                logger.info("原始图片识别结果为空")
        except Exception as e:
            logger.info(f"原始图片识别失败: {e}")
        
        # 方法2：使用预处理后的图片
        try:
            result2 = ocr.classification(processed_bytes)
            if result2 and result2.strip():
                results.append(('处理后图片', result2.strip()))
                logger.info(f"处理后图片识别结果: '{result2}'")
            else:
                logger.info("处理后图片识别结果为空")
        except Exception as e:
            logger.info(f"处理后图片识别失败: {e}")
        
        # 选择最佳结果
        if results:
            # 优先选择较长的结果（通常更完整）
            best_result = max(results, key=lambda x: len(x[1]))
            final_result = best_result[1]
            method = best_result[0]
            
            logger.info(f"验证码识别成功 ({method}): '{final_result}'")
            
            # 基本结果过滤
            final_result = final_result.replace(' ', '').replace('\n', '').replace('\t', '')
            
            return jsonify({
                'success': True,
                'code': final_result,
                'confidence': 0.95,
                'method': method
            })
        else:
            logger.warning("所有识别方法都失败了")
            return jsonify({
                'success': True,
                'code': '',  # 返回空字符串而不是错误，让上层处理
                'confidence': 0.0,
                'method': 'none'
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
        
        # 验证并处理图片数据
        try:
            from PIL import Image
            import io
            
            # 尝试打开图片验证格式
            img = Image.open(io.BytesIO(background_bytes))
            logger.info(f"Image info: format={img.format}, mode={img.mode}, size={img.size}")
            
            # 保存调试图片
            debug_path = f"tmp/ocr_debug_{int(time.time())}.png"
            img.save(debug_path, format='PNG')
            logger.info(f"Debug image saved to: {debug_path}")
            
        except Exception as e:
            logger.error(f"图片格式验证失败: {str(e)}")
        
        # 使用ddddocr的滑块识别功能
        # 创建滑块识别器，使用det模式尝试
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
            # 实现真正的图像识别算法找缺口
            logger.info("使用图像分析算法寻找缺口位置")
            
            from PIL import Image, ImageFilter
            import numpy as np
            
            img = Image.open(io.BytesIO(background_bytes))
            width, height = img.size
            logger.info(f"图片尺寸: {width}x{height}")
            
            # 方法1: 尝试修复ddddocr的图像格式问题
            try:
                # 确保图像格式正确
                if img.mode != 'RGB':
                    img = img.convert('RGB')
                
                # 保存为JPEG格式重新读取
                import io as temp_io
                jpeg_buffer = temp_io.BytesIO()
                img.save(jpeg_buffer, format='JPEG', quality=95)
                jpeg_bytes = jpeg_buffer.getvalue()
                
                res = slider_ocr.slide_comparison(jpeg_bytes)
                if res and 'target' in res and len(res['target']) > 0:
                    distance = res['target'][0]
                    logger.info(f"ddddocr识别成功(JPEG格式): {distance}px")
                else:
                    raise Exception("ddddocr返回空结果")
                    
            except Exception as e2:
                logger.warning(f"ddddocr JPEG尝试失败: {e2}")
                
                # 使用多种算法进行投票决策
                candidates = []
                
                # 方法1: 改进的边缘检测
                try:
                    edge_result = find_gap_with_edge_detection(img, width, height)
                    candidates.append(('边缘检测', edge_result))
                except Exception as e3:
                    logger.warning(f"边缘检测失败: {e3}")
                
                # 方法2: 改进的颜色分析
                try:
                    color_result = find_gap_with_color_analysis(img, width, height)
                    candidates.append(('颜色分析', color_result))
                except Exception as e4:
                    logger.warning(f"颜色分析失败: {e4}")
                
                # 方法3: 新增的拼图模板匹配
                try:
                    template_result = find_gap_with_puzzle_template_matching(img, width, height)
                    if isinstance(template_result, tuple):
                        # 如果返回元组，说明通过了验证
                        pos, verified = template_result
                        candidates.append(('拼图模板匹配', pos, verified))
                    else:
                        # 普通返回值
                        candidates.append(('拼图模板匹配', template_result, False))
                except Exception as e5:
                    logger.warning(f"拼图模板匹配失败: {e5}")
                
                # 智能决策系统：优先考虑通过验证的改进算法
                if candidates:
                    # 检查是否有通过验证的拼图模板匹配结果
                    template_verified = False
                    template_pos = None
                    
                    for candidate_data in candidates:
                        if len(candidate_data) >= 3 and candidate_data[0] == '拼图模板匹配':
                            method, pos, verified = candidate_data
                            if verified and width * 0.1 < pos < width * 0.9:
                                template_verified = True
                                template_pos = pos
                                break
                        elif len(candidate_data) == 2 and candidate_data[0] == '拼图模板匹配':
                            # 兼容旧格式
                            method, pos = candidate_data
                            if width * 0.1 < pos < width * 0.9:
                                template_pos = pos
                    
                    if template_verified and template_pos is not None:
                        # 改进算法通过验证，但需要与其他算法进行交叉验证
                        # 检查与颜色分析结果的一致性
                        color_pos = None
                        for candidate_data in candidates:
                            if len(candidate_data) >= 2 and candidate_data[0] == '颜色分析':
                                color_pos = candidate_data[1]
                                break
                        
                        if color_pos is not None:
                            pos_diff = abs(template_pos - color_pos)
                            if pos_diff <= 20:  # 缩小一致性范围到20px
                                # 两个算法接近时，更偏重拼图形状分析
                                final_pos = int(template_pos * 0.7 + color_pos * 0.3)
                                distance = final_pos
                                decision_method = f"算法一致性融合（{template_pos}px+{color_pos}px）"
                                logger.info(f"算法一致性融合: 拼图{template_pos}px + 颜色{color_pos}px = {distance}px")
                            elif pos_diff <= 50:  # 中等差距
                                # 平均加权
                                final_pos = int(template_pos * 0.5 + color_pos * 0.5)
                                distance = final_pos
                                decision_method = f"算法平衡融合（差距{pos_diff}px）"
                                logger.info(f"算法平衡融合: 拼图{template_pos}px + 颜色{color_pos}px = {distance}px")
                            else:
                                # 差距很大时，检查哪个更合理
                                # 偏向中央区域的算法结果
                                center_pos = width // 2
                                template_center_dist = abs(template_pos - center_pos)
                                color_center_dist = abs(color_pos - center_pos)
                                
                                if template_center_dist < color_center_dist:
                                    # 拼图算法更接近中央，给予更高权重
                                    final_pos = int(template_pos * 0.8 + color_pos * 0.2)
                                    decision_method = f"选择更居中的拼图算法（差距{pos_diff}px）"
                                else:
                                    # 颜色算法更接近中央
                                    final_pos = int(template_pos * 0.2 + color_pos * 0.8)
                                    decision_method = f"选择更居中的颜色算法（差距{pos_diff}px）"
                                
                                distance = final_pos
                                logger.info(f"大差距处理: 拼图{template_pos}px vs 颜色{color_pos}px → {distance}px")
                        else:
                            # 没有颜色分析结果时，使用原逻辑
                            distance = template_pos
                            decision_method = "拼图形状验证（高置信度）"
                            logger.info(f"使用经过验证的拼图形状识别结果: {distance}px")
                    else:
                        # 如果没有验证通过的结果，使用加权投票
                        # 检查拼图算法结果是否与其他算法差异过大
                        puzzle_pos = None
                        other_positions = []
                        for candidate_data in candidates:
                            if len(candidate_data) >= 2:
                                method, pos = candidate_data[0], candidate_data[1]
                                if method == '拼图模板匹配':
                                    puzzle_pos = pos
                                else:
                                    other_positions.append(pos)
                        
                        # 动态调整拼图算法权重
                        puzzle_weight = 3.0  # 默认高权重
                        if puzzle_pos is not None and other_positions:
                            other_avg = np.mean(other_positions)
                            pos_diff = abs(puzzle_pos - other_avg)
                            if pos_diff > 80:  # 差异太大，降低权重
                                puzzle_weight = 1.0
                                logger.info(f"拼图算法结果差异过大({pos_diff:.0f}px)，降低权重至1.0")
                            elif pos_diff > 50:  # 中等差异，中等权重
                                puzzle_weight = 1.8
                                logger.info(f"拼图算法结果有差异({pos_diff:.0f}px)，权重调整为1.8")
                        
                        weights = {
                            '边缘检测': 2.0,        # 边缘检测通常较可靠
                            '颜色分析': 2.5,        # 颜色分析较准确
                            '拼图模板匹配': puzzle_weight,  # 动态调整权重
                            '轮廓分析': 1.2         # 备用方法
                        }
                        
                        # 计算加权平均
                        weighted_sum = 0
                        total_weight = 0
                        positions = []
                        
                        for candidate_data in candidates:
                            if len(candidate_data) >= 2:
                                method = candidate_data[0]
                                pos = candidate_data[1]
                                weight = weights.get(method, 1.0)
                                weighted_sum += pos * weight
                                total_weight += weight
                                positions.append(pos)
                        
                        # 使用加权平均，但放宽偏差容忍度
                        weighted_avg = int(weighted_sum / total_weight) if total_weight > 0 else int(np.median(positions))
                        median_pos = int(np.median(positions))
                        
                        # 放宽偏差检查：从30px提高到60px
                        if abs(weighted_avg - median_pos) > 60:
                            distance = median_pos
                            decision_method = "中位数（极大偏差保护）"
                        else:
                            distance = weighted_avg
                            decision_method = "加权平均"
                    
                    # 日志记录所有候选结果和决策过程
                    candidate_info = []
                    for candidate_data in candidates:
                        if len(candidate_data) >= 2:
                            method, pos = candidate_data[0], candidate_data[1]
                            verified_mark = "✓" if len(candidate_data) >= 3 and candidate_data[2] else ""
                            candidate_info.append(f"{method}: {pos}px{verified_mark}")
                    logger.info(f"多算法候选位置: {', '.join(candidate_info)}")
                    if not template_verified:
                        logger.info(f"加权平均: {weighted_avg}px, 中位数: {median_pos}px")
                    logger.info(f"最终决策: {distance}px ({decision_method})")
                else:
                    # 所有算法都失败时的备用方案
                    distance = int(width * 0.3)  # 更合理的默认位置
                    logger.warning(f"所有算法失败，使用备用位置: {distance}px")
        
        logger.info(f"滑块验证码识别成功: 距离={distance}")
        
        return jsonify({
            'success': True,
            'distance': int(distance)  # 确保转换为标准Python int类型，避免numpy int64序列化错误
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
import React, { useState, useEffect } from 'react';
import { Modal, Form, Input, Button, Space, Tag, Radio, InputNumber, message } from 'antd';
import { SecurityScanOutlined, MobileOutlined, SlidersOutlined } from '@ant-design/icons';
import type { TestStep } from '../../types';

interface CaptchaMarkerProps {
  visible: boolean;
  step: TestStep | null;
  stepIndex: number;
  onClose: () => void;
  onMark: (step: TestStep) => void;
}

const CaptchaMarker: React.FC<CaptchaMarkerProps> = ({
  visible,
  step,
  stepIndex,
  onClose,
  onMark,
}) => {
  const [form] = Form.useForm();
  const [captchaType, setCaptchaType] = useState<string>('image_ocr');

  // 当步骤变化时更新表单字段
  useEffect(() => {
    if (step && visible) {
      form.setFieldsValue({
        captcha_type: 'image_ocr',
        captcha_selector: step.selector,
        input_selector: step.selector,
        slider_selector: step.selector,
        captcha_timeout: 60,
      });
      setCaptchaType('image_ocr');
    }
  }, [step, visible, form]);

  const handleClose = () => {
    form.resetFields();
    setCaptchaType('image_ocr');
    onClose();
  };

  const handleSubmit = () => {
    form.validateFields().then(values => {
      if (!step) return;
      
      const markedStep: TestStep = {
        ...step,
        is_captcha: true,
        captcha_type: values.captcha_type,
        captcha_selector: values.captcha_selector,
        captcha_input_selector: values.input_selector,
        captcha_phone: values.captcha_phone,
        captcha_timeout: values.captcha_timeout || 60,
      };
      
      onMark(markedStep);
      message.success('验证码步骤已标记');
      form.resetFields();
      setCaptchaType('image_ocr');
      onClose();
    });
  };

  return (
    <Modal
      title={
        <Space>
          <SecurityScanOutlined />
          标记验证码步骤
        </Space>
      }
      open={visible}
      onCancel={handleClose}
      footer={[
        <Button key="cancel" onClick={handleClose}>
          取消
        </Button>,
        <Button key="submit" type="primary" onClick={handleSubmit}>
          确认标记
        </Button>,
      ]}
      width={600}
    >
      {step && (
        <div style={{ marginBottom: 16 }}>
          <Tag color="blue">步骤 {stepIndex + 1}</Tag>
          <Tag color="green">{step.type}</Tag>
          <span style={{ marginLeft: 8 }}>{step.selector}</span>
        </div>
      )}

      <Form
        form={form}
        layout="vertical"
        initialValues={{
          captcha_type: 'image_ocr',
          captcha_timeout: 60,
        }}
      >
        <Form.Item
          name="captcha_type"
          label="验证码类型"
          rules={[{ required: true, message: '请选择验证码类型' }]}
        >
          <Radio.Group onChange={(e) => setCaptchaType(e.target.value)}>
            <Radio.Button value="image_ocr">
              <SecurityScanOutlined /> 图形验证码
            </Radio.Button>
            <Radio.Button value="sms">
              <MobileOutlined /> 短信验证码
            </Radio.Button>
            <Radio.Button value="sliding">
              <SlidersOutlined /> 滑块验证码
            </Radio.Button>
          </Radio.Group>
        </Form.Item>

        {captchaType === 'image_ocr' && (
          <>
            <Form.Item
              name="captcha_selector"
              label="验证码图片选择器"
              tooltip="CSS选择器，用于定位验证码图片元素"
              rules={[{ required: true, message: '请输入验证码图片选择器' }]}
            >
              <Input 
                placeholder="例如: img#captcha-image, .captcha-img" 
              />
            </Form.Item>

            <Form.Item
              name="input_selector"
              label="输入框选择器"
              tooltip="验证码输入框的CSS选择器"
            >
              <Input 
                placeholder="例如: input#captcha-input, input[name='captcha']" 
              />
            </Form.Item>
          </>
        )}

        {captchaType === 'sms' && (
          <>
            <Form.Item
              name="captcha_phone"
              label="接收短信的手机号"
              rules={[
                { required: true, message: '请输入手机号' },
                { pattern: /^1[3-9]\d{9}$/, message: '请输入有效的手机号' }
              ]}
            >
              <Input placeholder="例如: 13800138000" />
            </Form.Item>

            <Form.Item
              name="captcha_selector"
              label="发送验证码按钮选择器"
              tooltip="点击发送短信验证码的按钮"
            >
              <Input placeholder="例如: button.send-sms, #send-code-btn" />
            </Form.Item>

            <Form.Item
              name="input_selector"
              label="验证码输入框选择器"
              tooltip="输入短信验证码的输入框"
            >
              <Input 
                placeholder="例如: input#sms-code, input[name='smsCode']" 
              />
            </Form.Item>
          </>
        )}

        {captchaType === 'sliding' && (
          <>
            <Form.Item
              name="captcha_selector"
              label="滑块背景图选择器"
              tooltip="滑块验证码背景图片的CSS选择器"
              rules={[{ required: true, message: '请输入背景图选择器' }]}
            >
              <Input placeholder="例如: .slide-bg, canvas.captcha-bg" />
            </Form.Item>

            <Form.Item
              name="slider_selector"
              label="滑块按钮选择器"
              tooltip="需要拖动的滑块按钮"
            >
              <Input 
                placeholder="例如: .slide-btn, .slider-handle" 
              />
            </Form.Item>
          </>
        )}

        <Form.Item
          name="captcha_timeout"
          label="等待超时时间（秒）"
          tooltip="等待验证码的最大时间"
        >
          <InputNumber
            min={10}
            max={300}
            placeholder="默认60秒"
            style={{ width: '100%' }}
          />
        </Form.Item>
      </Form>

      <div style={{ marginTop: 16, padding: 12, background: '#f0f2f5', borderRadius: 4 }}>
        <div style={{ marginBottom: 8, fontWeight: 'bold' }}>使用说明：</div>
        {captchaType === 'image_ocr' && (
          <ul style={{ margin: 0, paddingLeft: 20 }}>
            <li>系统将自动截取验证码图片并通过OCR识别</li>
            <li>识别成功后自动填入验证码输入框</li>
            <li>支持常见的数字、字母验证码</li>
          </ul>
        )}
        {captchaType === 'sms' && (
          <ul style={{ margin: 0, paddingLeft: 20 }}>
            <li>需要连接Android手机并开启USB调试</li>
            <li>系统将自动读取短信并提取验证码</li>
            <li>请确保手机号正确且能接收短信</li>
          </ul>
        )}
        {captchaType === 'sliding' && (
          <ul style={{ margin: 0, paddingLeft: 20 }}>
            <li>系统将自动识别滑块缺口位置</li>
            <li>模拟人工拖动滑块到正确位置</li>
            <li>支持常见的滑块验证码组件</li>
          </ul>
        )}
      </div>
    </Modal>
  );
};

export default CaptchaMarker;
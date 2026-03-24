/*
 * Copyright 2026 The TPClaw Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package feishu

import (
	"encoding/json"
	"fmt"
	"testing"
)

func TestCardV2_SimpleMarkdown(t *testing.T) {
	card := NewCardV2().
		AddMarkdown("**Hello World**\n这是一条测试消息").
		Build()

	jsonStr, err := card.String()
	if err != nil {
		t.Fatalf("Failed to marshal card: %v", err)
	}

	fmt.Println("Simple Markdown Card:")
	fmt.Println(jsonStr)

	// 验证 JSON 结构
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		t.Fatalf("Invalid JSON: %v", err)
	}

	if result["schema"] != "2.0" {
		t.Error("Schema should be 2.0")
	}
}

func TestCardV2_ComplexCard(t *testing.T) {
	card := NewCardV2().
		Header(&CardHeader{
			Template_: "blue",
			Title_:    NewCardTitle().Content("卡片标题").Build(),
		}).
		AddMarkdown("**粗体文本** *斜体文本* ~~删除线~~").
		AddHr().
		AddDiv("这是一个分区文本").
		AddElement(NewCardAction().
			AddAction(NewCardButton().
				Text(NewCardPlainText().Content("点击按钮").Build()).
				URL("https://open.feishu.cn").
				Type("primary").
				Build()).
			AddAction(NewCardButton().
				Text(NewCardPlainText().Content("次要按钮").Build()).
				Type("default").
				Build()).
			Build()).
		AddHr().
		AddElement(NewCardNote().
			AddElement(NewCardPlainText().Content("这是备注信息").Build()).
			Build()).
		Build()

	jsonStr, err := card.String()
	if err != nil {
		t.Fatalf("Failed to marshal card: %v", err)
	}

	fmt.Println("\nComplex Card:")
	fmt.Println(jsonStr)
}

func TestCardV2_Container(t *testing.T) {
	card := NewCardV2().
		AddElement(NewCardContainer().
			Direction("horizontal").
			Padding("20px").
			AddElement(NewCardColumnSet().
				AddColumn(NewCardColumn().
					Width("50%").
					AddElement(NewCardMarkdown().Content("左侧内容").Build()).
					Build()).
				AddColumn(NewCardColumn().
					Width("50%").
					AddElement(NewCardMarkdown().Content("右侧内容").Build()).
					Build()).
				Build()).
			Build()).
		Build()

	jsonStr, err := card.String()
	if err != nil {
		t.Fatalf("Failed to marshal card: %v", err)
	}

	fmt.Println("\nContainer Card:")
	fmt.Println(jsonStr)
}

func TestCardV2_RichMarkdown(t *testing.T) {
	content := `标准emoji 😁😢🌞💼🏆❌✅
飞书emoji :OK::THUMBSUP:
*斜体* **粗体** ~~删除线~~
<font color='red'>这是红色文本</font>
<text_tag color="blue">标签</text_tag>
[文字链接](https://open.feishu.cn)
<link icon='chat_outlined' url='https://open.feishu.cn'>带图标的链接</link>
<at id=all></at>
- 无序列表1
    - 无序列表 1.1
- 无序列表2
1. 有序列表1
    1. 有序列表 1.1
2. 有序列表2
` + "```JSON\n{\"This is\": \"JSON demo\"}\n```"

	card := NewCardV2().
		AddMarkdown(content).
		Build()

	jsonStr, err := card.String()
	if err != nil {
		t.Fatalf("Failed to marshal card: %v", err)
	}

	fmt.Println("\nRich Markdown Card:")
	fmt.Println(jsonStr)

	// 验证生成的 JSON 符合 2.0 格式
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(jsonStr), &result); err != nil {
		t.Fatalf("Invalid JSON: %v", err)
	}

	if result["schema"] != "2.0" {
		t.Error("Schema should be 2.0")
	}

	body, ok := result["body"].(map[string]interface{})
	if !ok {
		t.Fatal("Body should be an object")
	}

	elements, ok := body["elements"].([]interface{})
	if !ok {
		t.Fatal("Elements should be an array")
	}

	if len(elements) != 1 {
		t.Fatalf("Should have 1 element, got %d", len(elements))
	}

	elem := elements[0].(map[string]interface{})
	if elem["tag"] != "markdown" {
		t.Error("Element tag should be markdown")
	}
}

func TestCardV2_ButtonWithConfirm(t *testing.T) {
	card := NewCardV2().
		AddElement(NewCardAction().
			AddAction(NewCardButton().
				Text(NewCardPlainText().Content("删除").Build()).
				Type("danger").
				Confirm(NewCardActionConfirm().
					Title(NewCardPlainText().Content("确认删除").Build()).
					Text(NewCardPlainText().Content("删除后数据将无法恢复，确定要删除吗？").Build()).
					Build()).
				Build()).
			Build()).
		Build()

	jsonStr, err := card.String()
	if err != nil {
		t.Fatalf("Failed to marshal card: %v", err)
	}

	fmt.Println("\nButton with Confirm:")
	fmt.Println(jsonStr)
}

func TestCardV2_MustString(t *testing.T) {
	card := NewCardV2().AddMarkdown("test")
	jsonStr := card.MustString()

	if jsonStr == "" {
		t.Error("MustString should return non-empty string")
	}

	fmt.Println("\nMustString result:")
	fmt.Println(jsonStr)
}

// 示例：如何在实际项目中使用
func ExampleCardV2() {
	// 简单使用
	card1 := NewCardV2().
		AddMarkdown("这是一条简单的消息").
		MustString()
	fmt.Println(card1)

	// 带标题的消息
	card2 := NewCardV2().
		Header(&CardHeader{
			Template_: "blue",
			Title_:    NewCardTitle().Content("通知标题").Build(),
		}).
		AddMarkdown("消息内容").
		MustString()
	fmt.Println(card2)

	// 带按钮的消息
	card3 := NewCardV2().
		AddMarkdown("请选择操作：").
		AddElement(NewCardAction().
			AddAction(NewCardButton().
				Text(NewCardPlainText().Content("确认").Build()).
				Type("primary").
				Build()).
			AddAction(NewCardButton().
				Text(NewCardPlainText().Content("取消").Build()).
				Type("default").
				Build()).
			Build()).
		MustString()
	fmt.Println(card3)
}

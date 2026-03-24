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
)

// CardV2 飞书卡片 JSON 2.0 结构
// 文档: https://open.feishu.cn/document/feishu-cards/card-json-v2-components/content-components/rich-text
type CardV2 struct {
	Schema_ string      `json:"schema"`
	Header_ *CardHeader `json:"header,omitempty"`
	Body_   *CardBody   `json:"body"`
	Footer_ *CardFooter `json:"footer,omitempty"`
}

// CardHeader 卡片头部
type CardHeader struct {
	Template_ string         `json:"template,omitempty"`
	Title_    *CardTitle     `json:"title,omitempty"`
	Subtitle_ *CardPlainText `json:"subtitle,omitempty"`
	Logo_     *CardIcon      `json:"logo,omitempty"`
	Extra_    CardV2Element  `json:"extra,omitempty"`
	Padding_  string         `json:"padding,omitempty"`
}

// CardBody 卡片主体
type CardBody struct {
	Direction_ string          `json:"direction,omitempty"`
	Padding_   string          `json:"padding,omitempty"`
	Spacing_   string          `json:"spacing,omitempty"`
	Elements_  []CardV2Element `json:"elements"`
}

// CardFooter 卡片底部
type CardFooter struct {
	Padding_  string          `json:"padding,omitempty"`
	Spacing_  string          `json:"spacing,omitempty"`
	Elements_ []CardV2Element `json:"elements"`
}

// CardV2Element 卡片元素接口
type CardV2Element interface {
	Tag() string
}

// ============== 基础组件 ==============

// CardMarkdown Markdown 组件
type CardMarkdown struct {
	Tag_       string `json:"tag"`
	Content_   string `json:"content"`
	TextSize_  string `json:"text_size,omitempty"`
	TextColor_ string `json:"text_color,omitempty"`
	TextAlign_ string `json:"text_align,omitempty"`
}

func NewCardMarkdown() *CardMarkdown {
	return &CardMarkdown{Tag_: "markdown"}
}

func (e *CardMarkdown) Content(content string) *CardMarkdown {
	e.Content_ = content
	return e
}

func (e *CardMarkdown) TextSize(size string) *CardMarkdown {
	e.TextSize_ = size
	return e
}

func (e *CardMarkdown) TextColor(color string) *CardMarkdown {
	e.TextColor_ = color
	return e
}

func (e *CardMarkdown) TextAlign(align string) *CardMarkdown {
	e.TextAlign_ = align
	return e
}

func (e *CardMarkdown) Build() *CardMarkdown {
	return e
}

func (e *CardMarkdown) Tag() string {
	return "markdown"
}

// CardPlainText 纯文本组件
type CardPlainText struct {
	Tag_       string `json:"tag"`
	Content_   string `json:"content"`
	TextSize_  string `json:"text_size,omitempty"`
	TextColor_ string `json:"text_color,omitempty"`
	TextAlign_ string `json:"text_align,omitempty"`
	Lines_     int    `json:"lines,omitempty"`
}

func NewCardPlainText() *CardPlainText {
	return &CardPlainText{Tag_: "plain_text"}
}

func (e *CardPlainText) Content(content string) *CardPlainText {
	e.Content_ = content
	return e
}

func (e *CardPlainText) TextSize(size string) *CardPlainText {
	e.TextSize_ = size
	return e
}

func (e *CardPlainText) TextColor(color string) *CardPlainText {
	e.TextColor_ = color
	return e
}

func (e *CardPlainText) TextAlign(align string) *CardPlainText {
	e.TextAlign_ = align
	return e
}

func (e *CardPlainText) Lines(lines int) *CardPlainText {
	e.Lines_ = lines
	return e
}

func (e *CardPlainText) Build() *CardPlainText {
	return e
}

func (e *CardPlainText) Tag() string {
	return "plain_text"
}

// CardTitle 标题组件
type CardTitle struct {
	Tag_       string    `json:"tag"`
	Content_   string    `json:"content"`
	TextSize_  string    `json:"text_size,omitempty"`
	TextColor_ string    `json:"text_color,omitempty"`
	TextAlign_ string    `json:"text_align,omitempty"`
	Icon_      *CardIcon `json:"icon,omitempty"`
}

func NewCardTitle() *CardTitle {
	return &CardTitle{Tag_: "title"}
}

func (e *CardTitle) Content(content string) *CardTitle {
	e.Content_ = content
	return e
}

func (e *CardTitle) TextSize(size string) *CardTitle {
	e.TextSize_ = size
	return e
}

func (e *CardTitle) TextColor(color string) *CardTitle {
	e.TextColor_ = color
	return e
}

func (e *CardTitle) TextAlign(align string) *CardTitle {
	e.TextAlign_ = align
	return e
}

func (e *CardTitle) Icon(icon *CardIcon) *CardTitle {
	e.Icon_ = icon
	return e
}

func (e *CardTitle) Build() *CardTitle {
	return e
}

func (e *CardTitle) Tag() string {
	return "title"
}

// CardDiv 分区组件
type CardDiv struct {
	Tag_    string         `json:"tag"`
	Text_   *CardPlainText `json:"text,omitempty"`
	Fields_ []*CardField   `json:"fields,omitempty"`
	Extra_  CardV2Element  `json:"extra,omitempty"`
}

func NewCardDiv() *CardDiv {
	return &CardDiv{Tag_: "div"}
}

func (e *CardDiv) Text(text *CardPlainText) *CardDiv {
	e.Text_ = text
	return e
}

func (e *CardDiv) Fields(fields []*CardField) *CardDiv {
	e.Fields_ = fields
	return e
}

func (e *CardDiv) AddField(field *CardField) *CardDiv {
	e.Fields_ = append(e.Fields_, field)
	return e
}

func (e *CardDiv) Extra(extra CardV2Element) *CardDiv {
	e.Extra_ = extra
	return e
}

func (e *CardDiv) Build() *CardDiv {
	return e
}

func (e *CardDiv) Tag() string {
	return "div"
}

// CardField 字段组件
type CardField struct {
	IsShort_ bool           `json:"is_short,omitempty"`
	Text_    *CardPlainText `json:"text,omitempty"`
}

func NewCardField() *CardField {
	return &CardField{}
}

func (f *CardField) IsShort(isShort bool) *CardField {
	f.IsShort_ = isShort
	return f
}

func (f *CardField) Text(text *CardPlainText) *CardField {
	f.Text_ = text
	return f
}

func (f *CardField) Build() *CardField {
	return f
}

// CardHr 分割线组件
type CardHr struct {
	Tag_ string `json:"tag"`
}

func NewCardHr() *CardHr {
	return &CardHr{Tag_: "hr"}
}

func (e *CardHr) Build() *CardHr {
	return e
}

func (e *CardHr) Tag() string {
	return "hr"
}

// CardNote 备注组件
type CardNote struct {
	Tag_      string          `json:"tag"`
	Elements_ []CardV2Element `json:"elements"`
}

func NewCardNote() *CardNote {
	return &CardNote{Tag_: "note"}
}

func (e *CardNote) Elements(elements []CardV2Element) *CardNote {
	e.Elements_ = elements
	return e
}

func (e *CardNote) AddElement(element CardV2Element) *CardNote {
	e.Elements_ = append(e.Elements_, element)
	return e
}

func (e *CardNote) Build() *CardNote {
	return e
}

func (e *CardNote) Tag() string {
	return "note"
}

// ============== 媒体组件 ==============

// CardImg 图片组件
type CardImg struct {
	Tag_          string         `json:"tag"`
	ImgKey_       string         `json:"img_key"`
	Alt_          *CardPlainText `json:"alt,omitempty"`
	Title_        CardV2Element  `json:"title,omitempty"`
	CustomWidth_  *int           `json:"custom_width,omitempty"`
	CompactWidth_ *bool          `json:"compact_width,omitempty"`
	Mode_         string         `json:"mode,omitempty"`
	Preview_      *bool          `json:"preview,omitempty"`
}

func NewCardImg() *CardImg {
	return &CardImg{Tag_: "img"}
}

func (e *CardImg) ImgKey(key string) *CardImg {
	e.ImgKey_ = key
	return e
}

func (e *CardImg) Alt(alt *CardPlainText) *CardImg {
	e.Alt_ = alt
	return e
}

func (e *CardImg) Title(title CardV2Element) *CardImg {
	e.Title_ = title
	return e
}

func (e *CardImg) CustomWidth(width int) *CardImg {
	e.CustomWidth_ = &width
	return e
}

func (e *CardImg) CompactWidth(compact bool) *CardImg {
	e.CompactWidth_ = &compact
	return e
}

func (e *CardImg) Mode(mode string) *CardImg {
	e.Mode_ = mode
	return e
}

func (e *CardImg) Preview(preview bool) *CardImg {
	e.Preview_ = &preview
	return e
}

func (e *CardImg) Build() *CardImg {
	return e
}

func (e *CardImg) Tag() string {
	return "img"
}

// CardIcon 图标组件
type CardIcon struct {
	Tag_    string `json:"tag"`
	ImgKey_ string `json:"img_key,omitempty"`
	Icon_   string `json:"icon,omitempty"`
	Color_  string `json:"color,omitempty"`
}

func NewCardIcon() *CardIcon {
	return &CardIcon{Tag_: "icon"}
}

func (e *CardIcon) ImgKey(key string) *CardIcon {
	e.ImgKey_ = key
	return e
}

func (e *CardIcon) Icon(icon string) *CardIcon {
	e.Icon_ = icon
	return e
}

func (e *CardIcon) Color(color string) *CardIcon {
	e.Color_ = color
	return e
}

func (e *CardIcon) Build() *CardIcon {
	return e
}

func (e *CardIcon) Tag() string {
	return "icon"
}

// ============== 交互组件 ==============

// CardAction 交互容器组件
type CardAction struct {
	Tag_     string          `json:"tag"`
	Layout_  string          `json:"layout,omitempty"`
	Spacing_ string          `json:"spacing,omitempty"`
	Actions_ []CardV2Element `json:"actions"`
}

func NewCardAction() *CardAction {
	return &CardAction{Tag_: "action"}
}

func (e *CardAction) Layout(layout string) *CardAction {
	e.Layout_ = layout
	return e
}

func (e *CardAction) Spacing(spacing string) *CardAction {
	e.Spacing_ = spacing
	return e
}

func (e *CardAction) Actions(actions []CardV2Element) *CardAction {
	e.Actions_ = actions
	return e
}

func (e *CardAction) AddAction(action CardV2Element) *CardAction {
	e.Actions_ = append(e.Actions_, action)
	return e
}

func (e *CardAction) Build() *CardAction {
	return e
}

func (e *CardAction) Tag() string {
	return "action"
}

// CardButton 按钮组件
type CardButton struct {
	Tag_      string                 `json:"tag"`
	Text_     CardV2Element          `json:"text,omitempty"`
	URL_      string                 `json:"url,omitempty"`
	MultiURL_ *CardMultiURL          `json:"multi_url,omitempty"`
	Type_     string                 `json:"type,omitempty"`
	Size_     string                 `json:"size,omitempty"`
	Width_    string                 `json:"width,omitempty"`
	Value_    map[string]interface{} `json:"value,omitempty"`
	Confirm_  *CardActionConfirm     `json:"confirm,omitempty"`
	Disabled_ *bool                  `json:"disabled,omitempty"`
}

func NewCardButton() *CardButton {
	return &CardButton{Tag_: "button"}
}

func (e *CardButton) Text(text CardV2Element) *CardButton {
	e.Text_ = text
	return e
}

func (e *CardButton) URL(url string) *CardButton {
	e.URL_ = url
	return e
}

func (e *CardButton) MultiURL(multiURL *CardMultiURL) *CardButton {
	e.MultiURL_ = multiURL
	return e
}

func (e *CardButton) Type(typ string) *CardButton {
	e.Type_ = typ
	return e
}

func (e *CardButton) Size(size string) *CardButton {
	e.Size_ = size
	return e
}

func (e *CardButton) Width(width string) *CardButton {
	e.Width_ = width
	return e
}

func (e *CardButton) Value(value map[string]interface{}) *CardButton {
	e.Value_ = value
	return e
}

func (e *CardButton) Confirm(confirm *CardActionConfirm) *CardButton {
	e.Confirm_ = confirm
	return e
}

func (e *CardButton) Disabled(disabled bool) *CardButton {
	e.Disabled_ = &disabled
	return e
}

func (e *CardButton) Build() *CardButton {
	return e
}

func (e *CardButton) Tag() string {
	return "button"
}

// CardMultiURL 多端链接
type CardMultiURL struct {
	URL_        string `json:"url,omitempty"`
	AndroidURL_ string `json:"android_url,omitempty"`
	IOSURL_     string `json:"ios_url,omitempty"`
	PCURL_      string `json:"pc_url,omitempty"`
}

func NewCardMultiURL() *CardMultiURL {
	return &CardMultiURL{}
}

func (u *CardMultiURL) URL(url string) *CardMultiURL {
	u.URL_ = url
	return u
}

func (u *CardMultiURL) AndroidURL(url string) *CardMultiURL {
	u.AndroidURL_ = url
	return u
}

func (u *CardMultiURL) IOSURL(url string) *CardMultiURL {
	u.IOSURL_ = url
	return u
}

func (u *CardMultiURL) PCURL(url string) *CardMultiURL {
	u.PCURL_ = url
	return u
}

func (u *CardMultiURL) Build() *CardMultiURL {
	return u
}

// CardActionConfirm 二次确认弹窗
type CardActionConfirm struct {
	Title_ *CardPlainText `json:"title,omitempty"`
	Text_  *CardPlainText `json:"text,omitempty"`
}

func NewCardActionConfirm() *CardActionConfirm {
	return &CardActionConfirm{}
}

func (c *CardActionConfirm) Title(title *CardPlainText) *CardActionConfirm {
	c.Title_ = title
	return c
}

func (c *CardActionConfirm) Text(text *CardPlainText) *CardActionConfirm {
	c.Text_ = text
	return c
}

func (c *CardActionConfirm) Build() *CardActionConfirm {
	return c
}

// ============== 容器组件 ==============

// CardContainer 容器组件
type CardContainer struct {
	Tag_        string          `json:"tag"`
	Direction_  string          `json:"direction,omitempty"`
	Padding_    string          `json:"padding,omitempty"`
	Spacing_    string          `json:"spacing,omitempty"`
	Background_ string          `json:"background,omitempty"`
	Elements_   []CardV2Element `json:"elements"`
}

func NewCardContainer() *CardContainer {
	return &CardContainer{Tag_: "container"}
}

func (e *CardContainer) Direction(direction string) *CardContainer {
	e.Direction_ = direction
	return e
}

func (e *CardContainer) Padding(padding string) *CardContainer {
	e.Padding_ = padding
	return e
}

func (e *CardContainer) Spacing(spacing string) *CardContainer {
	e.Spacing_ = spacing
	return e
}

func (e *CardContainer) Background(background string) *CardContainer {
	e.Background_ = background
	return e
}

func (e *CardContainer) Elements(elements []CardV2Element) *CardContainer {
	e.Elements_ = elements
	return e
}

func (e *CardContainer) AddElement(element CardV2Element) *CardContainer {
	e.Elements_ = append(e.Elements_, element)
	return e
}

func (e *CardContainer) Build() *CardContainer {
	return e
}

func (e *CardContainer) Tag() string {
	return "container"
}

// CardColumnSet 多列容器
type CardColumnSet struct {
	Tag_             string        `json:"tag"`
	Columns_         []*CardColumn `json:"columns"`
	FlexMode_        string        `json:"flex_mode,omitempty"`
	Background_      string        `json:"background,omitempty"`
	Width_           string        `json:"width,omitempty"`
	HorizontalAlign_ string        `json:"horizontal_align,omitempty"`
}

func NewCardColumnSet() *CardColumnSet {
	return &CardColumnSet{Tag_: "column_set"}
}

func (e *CardColumnSet) Columns(columns []*CardColumn) *CardColumnSet {
	e.Columns_ = columns
	return e
}

func (e *CardColumnSet) AddColumn(column *CardColumn) *CardColumnSet {
	e.Columns_ = append(e.Columns_, column)
	return e
}

func (e *CardColumnSet) FlexMode(mode string) *CardColumnSet {
	e.FlexMode_ = mode
	return e
}

func (e *CardColumnSet) Background(background string) *CardColumnSet {
	e.Background_ = background
	return e
}

func (e *CardColumnSet) Width(width string) *CardColumnSet {
	e.Width_ = width
	return e
}

func (e *CardColumnSet) HorizontalAlign(align string) *CardColumnSet {
	e.HorizontalAlign_ = align
	return e
}

func (e *CardColumnSet) Build() *CardColumnSet {
	return e
}

func (e *CardColumnSet) Tag() string {
	return "column_set"
}

// CardColumn 列组件
type CardColumn struct {
	Tag_             string          `json:"tag"`
	Width_           string          `json:"width,omitempty"`
	Weight_          int             `json:"weight,omitempty"`
	VerticalAlign_   string          `json:"vertical_align,omitempty"`
	HorizontalAlign_ string          `json:"horizontal_align,omitempty"`
	Elements_        []CardV2Element `json:"elements"`
}

func NewCardColumn() *CardColumn {
	return &CardColumn{Tag_: "column"}
}

func (e *CardColumn) Width(width string) *CardColumn {
	e.Width_ = width
	return e
}

func (e *CardColumn) Weight(weight int) *CardColumn {
	e.Weight_ = weight
	return e
}

func (e *CardColumn) VerticalAlign(align string) *CardColumn {
	e.VerticalAlign_ = align
	return e
}

func (e *CardColumn) HorizontalAlign(align string) *CardColumn {
	e.HorizontalAlign_ = align
	return e
}

func (e *CardColumn) Elements(elements []CardV2Element) *CardColumn {
	e.Elements_ = elements
	return e
}

func (e *CardColumn) AddElement(element CardV2Element) *CardColumn {
	e.Elements_ = append(e.Elements_, element)
	return e
}

func (e *CardColumn) Build() *CardColumn {
	return e
}

func (e *CardColumn) Tag() string {
	return "column"
}

// ============== 卡片构建器 ==============

// NewCardV2 创建卡片 JSON 2.0
func NewCardV2() *CardV2 {
	return &CardV2{
		Schema_: "2.0",
		Body_:   &CardBody{Elements_: []CardV2Element{}},
	}
}

// Header 设置卡片头部
func (c *CardV2) Header(header *CardHeader) *CardV2 {
	c.Header_ = header
	return c
}

// Body 设置卡片主体
func (c *CardV2) Body(body *CardBody) *CardV2 {
	c.Body_ = body
	return c
}

// Footer 设置卡片底部
func (c *CardV2) Footer(footer *CardFooter) *CardV2 {
	c.Footer_ = footer
	return c
}

// AddElement 添加元素到 Body
func (c *CardV2) AddElement(element CardV2Element) *CardV2 {
	if c.Body_ == nil {
		c.Body_ = &CardBody{Elements_: []CardV2Element{}}
	}
	c.Body_.Elements_ = append(c.Body_.Elements_, element)
	return c
}

// AddMarkdown 添加 Markdown 元素（快捷方法）
func (c *CardV2) AddMarkdown(content string) *CardV2 {
	return c.AddElement(NewCardMarkdown().Content(content).Build())
}

// AddPlainText 添加纯文本元素（快捷方法）
func (c *CardV2) AddPlainText(content string) *CardV2 {
	return c.AddElement(NewCardPlainText().Content(content).Build())
}

// AddHr 添加分割线（快捷方法）
func (c *CardV2) AddHr() *CardV2 {
	return c.AddElement(NewCardHr().Build())
}

// AddDiv 添加分区（快捷方法）
func (c *CardV2) AddDiv(text string) *CardV2 {
	return c.AddElement(NewCardDiv().Text(NewCardPlainText().Content(text).Build()).Build())
}

// Build 构建卡片
func (c *CardV2) Build() *CardV2 {
	return c
}

// String 转换为 JSON 字符串
func (c *CardV2) String() (string, error) {
	bs, err := json.Marshal(c)
	if err != nil {
		return "", err
	}
	return string(bs), nil
}

// MustString 转换为 JSON 字符串（忽略错误）
func (c *CardV2) MustString() string {
	s, _ := c.String()
	return s
}

// JSON 转换为 JSON 字符串（String 的别名）
func (c *CardV2) JSON() (string, error) {
	return c.String()
}

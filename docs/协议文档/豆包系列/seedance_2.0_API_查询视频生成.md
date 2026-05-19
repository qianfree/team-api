`GET https://ark.cn-beijing.volces.com/api/v3/contents/generations/tasks/{id}`  [运行](https://api.volcengine.com/api-explorer/?action=GetContentsGenerationsTask&data=%7B%22id%22%3A%22cgt-20250331175019-68d9t%22%7D&groupName=%E8%A7%86%E9%A2%91%E7%94%9F%E6%88%90API&query=%7B%7D&serviceCode=ark&version=2024-01-01)
查询视频生成任务的状态。
:::tip
仅支持查询最近 7 天的历史数据。时间计算统一采用UTC时间戳，返回的7天历史数据范围以用户实际发起查询请求的时刻为基准（精确到秒），时间戳区间为 [T\-7天, T)。

:::
```mixin-react
return (<Tabs>
<Tabs.TabPane title="快速入口" key="fq9yXaKY"><RenderMd content={` [ ](#)[体验中心](https://console.volcengine.com/ark/region:ark+cn-beijing/experience/vision)       <span>![图片](https://portal.volccdn.com/obj/volcfe/cloud-universal-doc/upload_2abecd05ca2779567c6d32f0ddc7874d.png =20x) </span>[模型列表](https://www.volcengine.com/docs/82379/1330310)       <span>![图片](https://portal.volccdn.com/obj/volcfe/cloud-universal-doc/upload_a5fdd3028d35cc512a10bd71b982b6eb.png =20x) </span>[模型计费](https://www.volcengine.com/docs/82379/1099320#%E8%A7%86%E9%A2%91%E7%94%9F%E6%88%90%E6%A8%A1%E5%9E%8B)       <span>![图片](https://portal.volccdn.com/obj/volcfe/cloud-universal-doc/upload_afbcf38bdec05c05089d5de5c3fd8fc8.png =20x) </span>[API Key](https://console.volcengine.com/ark/region:ark+cn-beijing/apiKey?apikey=%7B%7D)
 <span>![图片](https://portal.volccdn.com/obj/volcfe/cloud-universal-doc/upload_57d0bca8e0d122ab1191b40101b5df75.png =20x) </span>[调用教程](https://www.volcengine.com/docs/82379/1366799)       <span>![图片](https://portal.volccdn.com/obj/volcfe/cloud-universal-doc/upload_f45b5cd5863d1eed3bc3c81b9af54407.png =20x) </span>[接口文档](https://www.volcengine.com/docs/82379/1521309)       <span>![图片](https://portal.volccdn.com/obj/volcfe/cloud-universal-doc/upload_1609c71a747f84df24be1e6421ce58f0.png =20x) </span>[常见问题](https://www.volcengine.com/docs/82379/1359411)       <span>![图片](https://portal.volccdn.com/obj/volcfe/cloud-universal-doc/upload_bef4bc3de3535ee19d0c5d6c37b0ffdd.png =20x) </span>[开通模型](https://console.volcengine.com/ark/region:ark+cn-beijing/openManagement?LLM=%7B%7D&OpenTokenDrawer=false)
`}></RenderMd></Tabs.TabPane>
<Tabs.TabPane title="鉴权说明" key="3vCxpwty"><RenderMd content={`本接口支持 API Key 鉴权，详见[鉴权认证方式](https://www.volcengine.com/docs/82379/1298459)。
`}></RenderMd></Tabs.TabPane></Tabs>);
```


---


<span id="RxN8G2nH"></span>
## 请求参数
> 跳转 [响应参数](#7mi8G8RI)


---


**id** `string` %%require%%
您需要查询的视频生成任务的 ID 。
:::tip
上面参数为Query String Parameters，在URL String中传入。

:::
---


&nbsp;
<span id="7mi8G8RI"></span>
## 响应参数
> 跳转 [请求参数](#RxN8G2nH)


---


**id ** `string`
视频生成任务 ID 。

---


**model** `string`
任务使用的模型名称和版本，`模型名称-版本`。

---


**status** `string`
任务状态，以及相关的信息：

* `queued`：排队中。
* `running`：任务运行中。
* `cancelled`：取消任务，取消状态24h自动删除（只支持排队中状态的任务被取消）。
* `succeeded`： 任务成功。
* `failed`：任务失败。
* `expired`：任务超时。


---


**error** `object / null`
错误提示信息，任务成功返回`null`，任务失败时返回错误数据，错误信息具体参见 [错误处理](https://www.volcengine.com/docs/82379/1299023#.5pa56Iif6ZSZ6K-v56CB)。

属性

---


error.**code** `string`
错误码。

---


error.**message** `string`
错误提示信息。


---


**created_at** `integer`
任务创建时间的 Unix 时间戳（秒）。

---


**updated_at** `integer`
任务当前状态更新时间的 Unix 时间戳（秒）。

---


**content** `object`
视频生成任务的输出内容。

属性

---


content.**video_url** `string`
生成视频的 URL，格式为 mp4。为保障信息安全，生成的视频会在24小时后被清理，请及时转存。
推荐配置火山引擎 TOS 提供的数据订阅功能，将您的模型推理产物自动转存到自己的 TOS 桶中，便于长期备份或二次加工。详细介绍请参见 [TOS 数据订阅](https://www.volcengine.com/docs/6349/2280949?lang=zh)。
content.**last_frame_url ** `string`
视频的尾帧图像 URL。有效期为 24小时，请及时转存。
说明：[创建视频生成任务](https://www.volcengine.com/docs/82379/1520757) 时设置 `"return_last_frame": true` 时，会返回该参数。


---


**seed** `integer`
本次请求使用的种子整数值。

---


**resolution **  `string`
生成视频的分辨率。

---


**ratio ** `string`
生成视频的宽高比。

---


**duration** `integer`
生成视频的时长，单位：秒。
说明：**duration 和 frames 参数只会返回一个**。[创建视频生成任务](https://www.volcengine.com/docs/82379/1520757) 时未指定 frames，会返回 duration。

---


**frames** `integer`  
生成视频的帧数。
说明：**duration 和 frames 参数只会返回一个**。[创建视频生成任务](https://www.volcengine.com/docs/82379/1520757) 时指定了 frames，会返回 frames。

---


**framespersecond**  `integer`
生成视频的帧率。

---


**generate_audio** `boolean`
生成的视频是否包含与画面同步的声音。仅 seedance 2.0 & 2.0 fast、seedance 1.5 pro 会返回该参数。

* `true`：模型输出的视频包含同步音频。
* `false`：模型输出的视频为无声视频。


---


**tools==^new^==** ** ** `object[]`
本次请求模型实际使用的工具。未使用工具时不返回。

属性
tools.**type ** `string`
实际使用的工具类型

* web_search：联网搜索工具。


---


**safety_identifier==^new^==** `string`
终端用户的唯一标识符。若 [创建视频生成任务](https://www.volcengine.com/docs/82379/1520757) 时设置了该参数，接口会原样返回此信息。

---


**draft** `boolean`
生成的视频是否为 Draft 视频。仅 seedance 1.5 pro 会返回该参数。

* `true`：表示当前输出为 Draft 视频。
* `false`：表示当前输出为正常视频。


---


**draft_task_id ** `string`
Draft 视频任务 ID。基于 Draft 视频生成正式视频时，会返回该参数。

---


**service_tier  ** `string`
实际处理任务使用的服务等级。

---


**execution_expires_after** ** ** `integer`
任务超时阈值，单位：秒。

---


**usage** `object`
本次请求的 token 用量。

属性

---


usage.**completion_tokens** `integer`
模型输出视频花费的 token 数量，可作为计费对账依据。
:::tip
seedance 2.0 系列模型存在最低 token 用量限制，如果实际 token 用量 ＜ 最低 token 用量，本字段会返回最低 token 用量，平台按最低 token 用量计费。

:::
---


usage.**total_tokens** `integer`
本次请求消耗的总 token 数量。视频生成模型不统计输入 token，输入 token 为 0，故 **total_tokens**=**completion_tokens**。

---


usage.**tool_usage==^new^==** ** ** `object`
使用工具的用量信息。

属性
usage.tool_usage.**web_search ** `integer`
实际调用联网搜索工具的次数，仅开启联网搜索时返回。




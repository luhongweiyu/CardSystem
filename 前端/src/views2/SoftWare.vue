<template>
  <section class="页面" v-loading="加载中">
    <div class="页面标题行">
      <div>
        <h2>{{ 是代理账号 ? '软件设置' : '软件管理' }}</h2>
        <p class="说明">{{ 是代理账号 ? '查看管理员配置的软件和时长卡价格，设置自己的点卡代扣总开关。' : '客户端按分钟选择授权时长，实际扣点价格始终由服务端决定。' }}</p>
      </div>
      <div>
        <el-button :loading="加载中" :disabled="代理代扣.保存中" @click="刷新页面">刷新</el-button>
        <el-button v-if="!是代理账号" type="primary" @click="打开软件编辑">新增软件</el-button>
      </div>
    </div>

    <el-table :data="软件列表" border stripe row-key="ID">
      <el-table-column prop="ID" label="ID" width="70" />
      <el-table-column prop="Software" label="软件名称" min-width="170" />
      <el-table-column label="点卡方案" min-width="300">
        <template #default="scope">
          <span v-if="是代理账号 && !代理软件单价值(scope.row.ID)">未授权</span>
          <span v-else-if="点卡方案.加载中">加载中…</span>
          <span v-else-if="!点卡方案.已加载">暂不可用</span>
          <span v-else-if="!软件点卡方案(scope.row.ID).length">未配置</span>
          <div v-else class="方案列表">
            <el-tag
              v-for="item in 软件点卡方案(scope.row.ID)"
              :key="`${item.software}-${item.period_minutes}`"
              :type="!点卡计费周期有效(item.period_minutes) ? 'warning' : item.enabled ? 'success' : 'info'"
              effect="plain"
            >
              {{ 时长卡时长文本(item.period_minutes) }} / {{ item.cost }} 点{{ item.is_default ? ' · 默认' : '' }}{{ !item.enabled ? ' · 停用' : '' }}{{ !点卡计费周期有效(item.period_minutes) ? ' · 范围外' : '' }}
            </el-tag>
          </div>
        </template>
      </el-table-column>
      <el-table-column label="默认授权时长" width="130">
        <template #default="scope">{{ 时长卡时长文本(scope.row.default_period_minutes) }}</template>
      </el-table-column>
      <el-table-column v-if="!是代理账号" label="心跳间隔" width="130">
        <template #default="scope">{{ 周期文本(scope.row.heartbeat_interval_seconds) }}</template>
      </el-table-column>
      <el-table-column label="自动离线时间" width="140">
        <template #default="scope">{{ 时长卡时长文本(scope.row.online_grace_minutes || 60) }}</template>
      </el-table-column>
      <el-table-column label="点卡授权复用" width="120">
        <template #default="scope">
          <el-tag :type="scope.row.point_card_reuse_enabled ? 'success' : 'info'">
            {{ scope.row.point_card_reuse_enabled ? '允许' : '关闭' }}
          </el-tag>
        </template>
      </el-table-column>
      <el-table-column label="暂停扣除" width="120">
        <template #default="scope">{{ scope.row.pause_deduct_minutes ? 时长卡时长文本(scope.row.pause_deduct_minutes) : '0 分钟' }}</template>
      </el-table-column>
      <el-table-column v-if="是代理账号" label="点卡每点价格" width="130">
        <template #default="scope">{{ 代理软件单价(scope.row.ID) }}</template>
      </el-table-column>
      <el-table-column prop="Bulletin" label="公告" min-width="220" show-overflow-tooltip />
      <el-table-column v-if="!是代理账号" label="操作" width="230" fixed="right">
        <template #default="scope">
          <el-button link type="primary" @click="编辑软件(scope.row)">编辑</el-button>
          <el-button link type="success" @click="打开价格(scope.row)">点卡计费方案</el-button>
          <el-button link type="danger" @click="删除软件(scope.row)">删除</el-button>
        </template>
      </el-table-column>
    </el-table>

    <div v-if="是代理账号" class="代理设置区">
      <div class="代扣设置栏" v-loading="代理代扣.加载中">
        <div class="代扣设置主行">
          <span class="代扣设置名称">点卡代扣</span>
          <span class="设置说明">卡内点数不足时使用账户余额</span>
          <el-switch
            v-model="代理代扣.point_card_auto_deduct"
            active-text="已开启"
            inactive-text="已关闭"
            :disabled="代理代扣.加载中 || 代理代扣.保存中"
          />
          <el-button size="small" type="primary" :loading="代理代扣.保存中" :disabled="!代理代扣有改动 || 代理代扣.加载中" @click="保存代理代扣">保存</el-button>
          <el-button link size="small" class="账户详情切换" :aria-expanded="显示账户详情" @click="显示账户详情 = !显示账户详情">
            {{ 显示账户详情 ? '收起账户详情' : '查看账户详情' }}
          </el-button>
        </div>
        <div v-if="显示账户详情" class="代理设置状态">
          <span>当前余额：{{ stores.账号信息.balance ?? 0 }} 点</span>
          <span>余额下限：{{ 代理代扣.min_balance }} 点</span>
        </div>
      </div>

      <el-card shadow="never" v-loading="代理价格加载中">
        <div class="代理设置标题">
          <div>
            <h3>时长卡价格</h3>
            <p class="设置说明">精确命中锚点使用原价，其他时长按相邻锚点规则计价。</p>
          </div>
          <el-button size="small" @click="查询代理价格(true)">刷新价格</el-button>
        </div>
        <el-form :inline="true" @submit.prevent>
          <el-form-item label="软件">
            <el-select v-model="代理价格软件" placeholder="全部软件" style="width: 220px">
              <el-option label="全部软件" :value="0" />
              <el-option v-for="item in 代理价格软件列表" :key="item.ID" :label="item.Software" :value="item.ID" />
            </el-select>
          </el-form-item>
        </el-form>
        <div v-if="代理时长价格分组.length" class="时长价格分组">
          <div v-for="group in 代理时长价格分组" :key="group.software" class="时长价格组">
            <div class="时长价格组标题">{{ group.name }}</div>
            <div class="方案列表">
              <el-tag v-for="item in group.prices" :key="item.id" effect="plain">
                {{ 时长卡时长文本(item.duration_minutes) }} / {{ Number(item.price).toFixed(2) }} 点{{ item.duration_minutes === 最大时长分钟 ? ' · 永久卡' : '' }}
              </el-tag>
            </div>
          </div>
        </div>
        <el-empty v-else-if="!代理价格加载中" :description="代理价格软件 ? '该软件暂无可用的时长卡价格' : '管理员尚未配置可用的时长卡价格'" />
      </el-card>
    </div>

    <section v-if="!是代理账号" class="代理区">
      <div class="子标题行">
        <h3>渠道合伙人</h3>
        <div>
          <el-button plain @click="打开代理时长价格总览">时长价格总览</el-button>
          <el-button type="primary" plain @click="打开代理创建">新增渠道合伙人</el-button>
        </div>
      </div>
      <el-table :data="代理列表" border stripe>
        <el-table-column prop="id" label="ID" width="70" />
        <el-table-column prop="name" label="账号" width="160" />
        <el-table-column prop="balance" label="余额（点）" width="120" />
        <el-table-column label="点卡每点价格" min-width="300">
          <template #default="scope">
            <span v-if="!代理软件价格列表(scope.row).length">未配置</span>
            <div v-else class="方案列表">
              <el-tooltip v-if="代理统一单价(scope.row)" :content="代理软件价格列表(scope.row).map((item) => item.name).join('、')" placement="top">
                <el-tag effect="plain">
                  {{ 代理软件价格列表(scope.row).length === 软件列表.length ? '全部软件' : `已授权 ${代理软件价格列表(scope.row).length}/${软件列表.length} 个软件` }}：{{ 代理统一单价(scope.row).toFixed(2) }} 点/点
                </el-tag>
              </el-tooltip>
              <template v-else>
                <el-tag v-for="item in 代理软件价格列表(scope.row)" :key="item.id" effect="plain">
                  {{ item.name }}：{{ item.price.toFixed(2) }} 点/点
                </el-tag>
              </template>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="余额下限（点）" width="145">
          <template #default="scope">{{ scope.row.min_balance }}</template>
        </el-table-column>
        <el-table-column label="操作" min-width="410">
          <template #default="scope">
            <el-button link type="primary" @click="编辑代理(scope.row)">计费方案与密码</el-button>
            <el-button link type="warning" @click="打开代理时长价格(scope.row)">时长卡价格</el-button>
            <el-button link type="success" @click="打开代理充值(scope.row)">调整余额</el-button>
            <el-button link type="primary" @click="打开代理日志(scope.row)">查看日志</el-button>
            <el-button link type="danger" @click="删除代理(scope.row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
    </section>

    <!-- 软件编辑 -->
    <el-dialog v-model="软件框.显示" :title="软件框.id ? '编辑软件' : '新增软件'" width="500px" destroy-on-close>
      <el-form label-width="125px" v-loading="软件框.加载中">
        <el-form-item label="软件名称" required>
          <el-input v-model="软件框.software" maxlength="64" />
        </el-form-item>
        <el-form-item label="默认授权时长（分钟）" required>
          <el-input-number
            v-model="软件框.default_period_minutes"
            :min="最小计费周期分钟"
            :max="最大计费周期分钟"
            :precision="0"
            controls-position="right"
          />
          <div v-if="软件框.default_period_minutes >= 60" class="字段说明">{{ 时长卡时长文本(软件框.default_period_minutes) }}</div>
        </el-form-item>
        <el-form-item label="心跳间隔（分钟）" required>
          <el-input-number
            v-model="软件框.heartbeat_interval_minutes"
            :min="0"
            :max="1440"
            controls-position="right"
          />
          <div class="字段说明">可输入小数分钟，最小 1 秒</div>
        </el-form-item>
        <el-form-item label="自动离线时间（分钟）" required>
          <el-input-number
            v-model="软件框.online_grace_minutes"
            :min="0"
            :max="最大自动离线时间分钟"
            :precision="0"
            controls-position="right"
          />
          <div class="字段说明">0 表示默认 60 分钟后自动离线</div>
          <div v-if="软件框.online_grace_minutes >= 60" class="字段说明">{{ 时长卡时长文本(软件框.online_grace_minutes) }}</div>
          <div v-if="软件框.heartbeat_interval_minutes >= (软件框.online_grace_minutes || 60)" class="字段说明 风险提醒">心跳间隔不小于自动离线时间，设备可能被误判离线。</div>
          <div v-if="(软件框.online_grace_minutes || 60) > 软件框.default_period_minutes" class="字段说明 风险提醒">自动离线时间长于默认授权周期，断线后可能连续多次续费扣点。</div>
        </el-form-item>
        <el-form-item label="暂停扣除（分钟）" required>
          <el-input-number
            v-model="软件框.pause_deduct_minutes"
            :min="0"
            :max="最大时长分钟"
            :precision="0"
            controls-position="right"
          />
          <div class="字段说明">0 表示不启用时长卡暂停；暂停时从剩余时长中扣除</div>
          <div v-if="软件框.pause_deduct_minutes >= 60" class="字段说明">{{ 时长卡时长文本(软件框.pause_deduct_minutes) }}</div>
        </el-form-item>
        <el-form-item label="点卡授权复用">
          <el-switch v-model="软件框.point_card_reuse_enabled" />
          <div class="字段说明">允许客户端主动请求接手同卡离线设备的未到期授权；未请求时仍按原规则计费。</div>
        </el-form-item>
        <el-form-item label="公告">
          <el-input v-model="软件框.bulletin" type="textarea" :rows="4" maxlength="5000" show-word-limit />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="软件框.显示 = false">取消</el-button>
        <el-button type="primary" :loading="软件框.加载中" @click="保存软件">保存</el-button>
      </template>
    </el-dialog>

    <!-- 点卡计费方案 -->
    <el-dialog v-model="价格框.显示" title="点卡计费方案" width="760px" destroy-on-close>
      <div class="价格标题">
        <span>{{ 价格框.softwareName }}</span>
        <el-button type="primary" size="small" :disabled="价格框.加载中 || 价格框.保存中 || 价格框.rows.some((item) => !item.id)" @click="新增价格">新增计费方案</el-button>
      </div>
      <el-alert
        v-if="价格框.rows.some((item) => !点卡计费周期有效(item.period_minutes))"
        title="旧范围方案已不可用；若它是默认方案，请先新增方案并设为默认，再删除旧方案。"
        type="warning"
        :closable="false"
        class="代理价格提示"
      />
      <el-table v-loading="价格框.加载中 || 价格框.保存中" :data="价格框.rows" border>
        <el-table-column label="授权时长" min-width="195">
          <template #default="scope">
            <template v-if="scope.row.id">{{ 时长卡时长文本(scope.row.period_minutes) }}</template>
            <div v-else>
              <el-input-number v-model="scope.row.period_minutes" :min="最小计费周期分钟" :max="最大计费周期分钟" :precision="0" :disabled="scope.row.保存中" controls-position="right" class="方案时长输入" />
              <span class="价格单位">{{ 时长卡时长文本(scope.row.period_minutes) }}</span>
            </div>
            <div v-if="scope.row.period_minutes < 价格框.onlineGraceMinutes" class="风险提醒 方案提醒">短于自动离线时间</div>
          </template>
        </el-table-column>
        <el-table-column label="扣点" width="150">
          <template #default="scope">
            <el-input-number v-if="点卡计费周期有效(scope.row.period_minutes)" v-model="scope.row.cost" :min="1" :max="1000000000" :precision="0" :disabled="scope.row.保存中" controls-position="right" class="方案扣点输入" />
            <span v-else>{{ scope.row.cost }}</span>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="90">
          <template #default="scope">
            <el-tag v-if="!点卡计费周期有效(scope.row.period_minutes)" type="warning">范围外</el-tag>
            <el-switch v-else v-model="scope.row.enabled" :disabled="scope.row.原默认 || scope.row.is_default || scope.row.保存中" />
          </template>
        </el-table-column>
        <el-table-column label="默认方案" width="100">
          <template #default="scope">
            <el-tag v-if="scope.row.原默认" type="success">默认</el-tag>
            <el-switch v-else v-model="scope.row.is_default" :disabled="!scope.row.enabled || !点卡计费周期有效(scope.row.period_minutes) || scope.row.保存中" />
          </template>
        </el-table-column>
        <el-table-column label="操作" width="130">
          <template #default="scope">
            <el-button link type="primary" :loading="scope.row.保存中" :disabled="!点卡计费周期有效(scope.row.period_minutes)" @click="保存价格(scope.row)">保存</el-button>
            <el-button link type="danger" :disabled="scope.row.原默认 || scope.row.保存中" @click="删除价格(scope.row)">{{ scope.row.id ? '删除' : '取消' }}</el-button>
          </template>
        </el-table-column>
      </el-table>
      <el-empty v-if="!价格框.加载中 && !价格框.rows.length" description="尚未配置点卡计费方案" />
    </el-dialog>

    <!-- 渠道合伙人 -->
    <el-dialog v-model="代理框.显示" title="新增渠道合伙人" width="420px" destroy-on-close>
      <el-form label-width="100px">
        <el-form-item label="账号"><el-input v-model="代理框.name" maxlength="32" /></el-form-item>
        <el-form-item label="密码">
          <el-input v-model="代理框.password" type="password" show-password />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="代理框.显示 = false">取消</el-button>
        <el-button type="primary" @click="创建代理">创建</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="代理编辑框.显示" title="渠道合伙人设置" width="620px" destroy-on-close>
      <p>账号：{{ 代理编辑框.name }}　当前余额：{{ 代理编辑框.balance }} 点</p>
      <el-form label-width="150px">
        <el-form-item v-for="item in 软件列表" :key="item.ID" :label="`${item.Software}（每点价格）`">
          <el-input-number
            v-model="代理编辑框.prices[item.ID]"
            :min="0"
            :max="1000000000"
            :precision="2"
            controls-position="right"
          />
          <span class="价格说明">0 表示不授权该软件</span>
        </el-form-item>
        <el-form-item label="余额下限">
          <el-input-number
            v-model="代理编辑框.min_balance"
            :min="-最大代理余额下限绝对值"
            :max="最大代理余额下限绝对值"
            :precision="0"
            controls-position="right"
          />
          <span class="价格说明">点；负数可欠费，0 不欠费，正数须保留相应余额</span>
        </el-form-item>
        <el-form-item label="新密码（可选）">
          <el-input v-model="代理编辑框.password" type="password" show-password placeholder="留空表示不修改" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="代理编辑框.显示 = false">取消</el-button>
        <el-button type="primary" @click="保存代理">保存</el-button>
      </template>
    </el-dialog>

    <!-- 时长卡代理价格：按代理账号和软件整组维护价格锚点。 -->
    <el-dialog v-model="代理时长价格框.显示" title="时长卡价格设置" width="780px" destroy-on-close>
      <div class="代理价格标题">
        <span>小伙伴：{{ 代理时长价格框.name }}</span>
        <el-select
          v-model="代理时长价格框.software"
          placeholder="请选择软件"
          style="width: 230px"
          @change="查询代理时长价格"
        >
          <el-option v-for="item in 软件列表" :key="item.ID" :label="item.Software" :value="item.ID" />
        </el-select>
      </div>
      <el-alert
        class="代理价格提示"
        type="info"
        :closable="false"
        title="精确命中锚点使用原价；锚点之间按两侧较高的平均每分钟价格折算。"
      />
      <el-table v-loading="代理时长价格框.加载中" :data="代理时长价格框.rows" border>
        <el-table-column label="卡面时长" min-width="190">
          <template #default="scope">
            <el-input-number
              v-model="scope.row.duration_minutes"
              :min="最小时长分钟"
              :max="最大时长分钟"
              :precision="0"
              controls-position="right"
            />
            <span class="价格单位">{{ 时长卡时长文本(scope.row.duration_minutes) }}</span>
          </template>
        </el-table-column>
        <el-table-column label="代理价格（点）" width="190">
          <template #default="scope">
            <el-input-number
              v-model="scope.row.price"
              :min="0.01"
              :max="最大代理价格"
              :precision="2"
              controls-position="right"
            />
          </template>
        </el-table-column>
        <el-table-column label="状态" width="100">
          <template #default="scope">
            <el-switch v-model="scope.row.enabled" />
          </template>
        </el-table-column>
        <el-table-column label="操作" width="90">
          <template #default="scope">
            <el-button link type="danger" @click="删除代理时长价格(scope.$index, scope.row)">删除</el-button>
          </template>
        </el-table-column>
      </el-table>
      <el-empty v-if="!代理时长价格框.rows.length && !代理时长价格框.加载中" description="尚未配置时长卡价格" />
      <template #footer>
        <el-button @click="代理时长价格框.显示 = false">取消</el-button>
        <el-button :disabled="代理时长价格框.rows.length >= 50" @click="新增代理时长价格">新增价格锚点</el-button>
        <el-button type="primary" :loading="代理时长价格框.保存中" @click="保存代理时长价格">保存整组价格</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="代理时长价格总览框.显示" title="时长卡价格总览" width="1100px" destroy-on-close>
      <el-form :inline="true" :disabled="代理时长价格总览框.加载中" @submit.prevent>
        <el-form-item label="代理">
          <el-select v-model="代理时长价格总览框.agent_id" clearable placeholder="全部代理" style="width: 190px" @change="查询代理时长价格总览">
            <el-option label="全部代理" :value="0" />
            <el-option v-for="item in 代理列表" :key="item.id" :label="item.name" :value="item.id" />
          </el-select>
        </el-form-item>
        <el-form-item label="软件">
          <el-select v-model="代理时长价格总览框.software" clearable placeholder="全部软件" style="width: 190px" @change="查询代理时长价格总览">
            <el-option label="全部软件" :value="0" />
            <el-option v-for="item in 软件列表" :key="item.ID" :label="item.Software" :value="item.ID" />
          </el-select>
        </el-form-item>
        <el-form-item>
          <el-button @click="查询代理时长价格总览">刷新</el-button>
        </el-form-item>
      </el-form>
      <el-table v-loading="代理时长价格总览框.加载中" :data="代理时长价格总览框.rows" row-key="key" border>
        <el-table-column prop="agent_name" label="代理" width="180" />
        <el-table-column prop="software_name" label="软件" width="180" />
        <el-table-column label="价格锚点" min-width="620">
          <template #default="scope">
            <template v-if="scope.row.prices.length">
              <div v-for="(price, priceIndex) in scope.row.prices" :key="price.key" class="总览价格">
                <el-input-number v-model="price.duration_minutes" :min="最小时长分钟" :max="最大时长分钟" :precision="0" size="small" controls-position="right" />
                <span>({{ 时长卡时长文本(price.duration_minutes) }})：</span>
                <el-input-number v-model="price.price" :min="0.01" :max="最大代理价格" :precision="2" size="small" controls-position="right" />
                <span>点</span>
                <el-switch v-model="price.enabled" size="small" />
                <el-button link type="danger" @click="删除总览锚点(scope.row, priceIndex)">删除</el-button>
              </div>
            </template>
            <span v-else class="弱文本">尚未配置</span>
            <div>
              <el-button size="small" plain :disabled="scope.row.保存中 || scope.row.prices.length >= 50" @click="新增总览锚点(scope.row)">新增锚点</el-button>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="80">
          <template #default="scope">
            <el-button link type="primary" :loading="scope.row.保存中" @click="保存总览价格(scope.row)">保存</el-button>
          </template>
        </el-table-column>
      </el-table>
      <el-empty v-if="!代理时长价格总览框.rows.length && !代理时长价格总览框.加载中" description="暂无时长卡价格" />
    </el-dialog>

    <el-dialog v-model="代理充值框.显示" title="调整渠道合伙人余额" width="420px" destroy-on-close>
      <p>账号：{{ 代理充值框.name }}</p>
      <el-form label-width="90px">
        <el-form-item label="变更点数">
          <el-input-number
            v-model="代理充值框.amount"
            :precision="0"
            controls-position="right"
          />
        </el-form-item>
        <p class="提示文字">正数为充值，负数为扣款；请输入非零点数，余额允许变为负数。</p>
        <el-form-item label="备注"><el-input v-model="代理充值框.note" maxlength="200" /></el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="代理充值框.显示 = false">取消</el-button>
        <el-button type="primary" :loading="代理充值框.保存中" :disabled="!代理充值框.amount" @click="代理充值">确认调整</el-button>
      </template>
    </el-dialog>

    <el-dialog v-model="代理日志框.显示" :title="`渠道合伙人日志 · ${代理日志框.name}`" width="850px" destroy-on-close>
      <p class="设置说明">最近两个月的日志；最新记录在上方。</p>
      <div v-loading="代理日志框.加载中" class="代理日志内容">
        <pre v-if="代理日志框.content">{{ 代理日志框.content }}</pre>
        <el-empty v-else-if="!代理日志框.加载中" description="暂无日志" />
      </div>
      <template #footer>
        <el-button :loading="代理日志框.加载中" @click="查询代理日志">刷新</el-button>
        <el-button @click="代理日志框.显示 = false">关闭</el-button>
      </template>
    </el-dialog>
  </section>
</template>

<script setup>
import { computed, onMounted, reactive, ref } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { storeToRefs } from 'pinia'
import { use登录状态Store } from '../stores/登录状态.js'
import { 获取接口错误提示 } from '../api/请求客户端.js'
import { 日志倒序 } from '../utils/日志工具.js'
import {
  格式化时长 as 时长卡时长文本,
  最大计费周期分钟,
  最大时长分钟,
  最小计费周期分钟,
  最小时长分钟
} from '../utils/时长工具.js'

const 最大代理价格 = 1000000000
const 最大代理余额下限绝对值 = 1000000000
// 自动离线时间仍独立允许 5 分钟至 3 天，不能随点卡计费周期一起扩到 30 天。
const 最小自动离线时间分钟 = 5
const 最大自动离线时间分钟 = 3 * 24 * 60

const stores = use登录状态Store()
const post = stores.post
const { 软件列表, 代理列表, 代理时长价格列表: 代理价格列表 } = storeToRefs(stores)
const 是代理账号 = computed(() => Boolean(stores.是代理账号))
const 加载中 = ref(false)
const 代理代扣 = reactive({
  加载中: false,
  保存中: false,
  point_card_auto_deduct: false,
  已保存代扣: false,
  min_balance: 0
})
const 代理代扣有改动 = computed(() => 代理代扣.point_card_auto_deduct !== 代理代扣.已保存代扣)
const 代理价格加载中 = ref(false)
const 代理价格软件 = ref(0)
const 显示账户详情 = ref(false)
const 点卡方案 = reactive({ 加载中: false, 已加载: false, rows: [] })
const 软件框 = reactive({
  显示: false,
  加载中: false,
  id: 0,
  software: '',
  bulletin: '',
  default_period_minutes: 60,
  heartbeat_interval_minutes: 5,
  online_grace_minutes: 60,
  point_card_reuse_enabled: true,
  pause_deduct_minutes: 0
})
const 价格框 = reactive({ 显示: false, 加载中: false, 保存中: false, software: 0, softwareName: '', onlineGraceMinutes: 60, rows: [] })
const 代理框 = reactive({ 显示: false, name: '', password: '' })
const 代理编辑框 = reactive({
  显示: false,
  id: 0,
  name: '',
  balance: 0,
  prices: {},
  password: '',
  min_balance: 0
})
const 代理时长价格框 = reactive({
  显示: false,
  加载中: false,
  保存中: false,
  agent_id: 0,
  name: '',
  software: 0,
  rows: []
})
const 代理时长价格总览框 = reactive({ 显示: false, 加载中: false, agent_id: 0, software: 0, rows: [] })
const 代理充值框 = reactive({ 显示: false, 保存中: false, id: 0, name: '', amount: 0, note: '' })
const 代理日志框 = reactive({ 显示: false, 加载中: false, id: 0, name: '', content: '' })

const 代理价格软件列表 = computed(() => {
  const ids = new Set(代理价格列表.value.map((item) => Number(item.software)))
  return 软件列表.value.filter((item) => ids.has(Number(item.ID)))
})
const 当前代理价格 = computed(() => {
  return [...代理价格列表.value]
    .filter((item) => !代理价格软件.value || Number(item.software) === Number(代理价格软件.value))
    .sort((a, b) => Number(a.software) - Number(b.software) || Number(a.duration_minutes) - Number(b.duration_minutes))
})
const 代理时长价格分组 = computed(() => {
  const groups = new Map()
  for (const item of 当前代理价格.value) {
    const software = Number(item.software)
    if (!groups.has(software)) groups.set(software, { software, name: 软件名称(software), prices: [] })
    groups.get(software).prices.push(item)
  }
  return [...groups.values()]
})

const 显示错误 = (error) => ElMessage.error(获取接口错误提示(error))
const 点卡计费周期有效 = (minutes) => Number.isInteger(minutes) && minutes >= 最小计费周期分钟 && minutes <= 最大计费周期分钟
const 周期文本 = function (seconds) {
  const value = Number(seconds || 0)
  if (!value) return '-'
  if (value < 60) return `${value}秒`
  const remainder = value % 60
  return `${时长卡时长文本(Math.floor(value / 60))}${remainder ? `${remainder}秒` : ''}`
}
const 代理软件单价值 = function (id) {
  const prices = stores.账号信息.prices || {}
  const value = Number(prices[String(id)] ?? prices[id] ?? 0)
  return Number.isFinite(value) && value > 0 ? value : 0
}
const 代理软件单价 = function (id) {
  const value = 代理软件单价值(id)
  return value ? `${value.toFixed(2)} 点` : '未授权'
}
const 软件名称 = function (id) {
  return 软件列表.value.find((item) => Number(item.ID) === Number(id))?.Software || `软件#${id}`
}
const 软件点卡方案 = (id) => 点卡方案.rows.filter((item) => Number(item.software) === Number(id))
// 编辑框以分钟显示；保留四位小数足以往返还原已有的整秒配置。
const 心跳秒转分钟 = (seconds) => Number((Number(seconds) / 60).toFixed(4))
const 代理软件价格列表 = function (row) {
  let prices = row.prices || {}
  try {
    if (typeof prices === 'string') prices = JSON.parse(prices)
  } catch {
    prices = {}
  }
  return 软件列表.value.flatMap((item) => {
    const price = Number(prices?.[item.ID] ?? 0)
    return Number.isFinite(price) && price > 0 ? [{ id: item.ID, name: item.Software, price }] : []
  })
}
// 仅在两个及以上已授权软件同价时合并；部分授权必须显示覆盖数量，不能写成“全部软件”。
const 代理统一单价 = function (row) {
  const items = 代理软件价格列表(row)
  return items.length > 1 && items.every((item) => item.price === items[0].price) ? items[0].price : null
}
const 同步代理代扣设置 = function (data = {}) {
  if (data.point_card_auto_deduct !== undefined) {
    代理代扣.已保存代扣 = Boolean(data.point_card_auto_deduct)
    代理代扣.point_card_auto_deduct = 代理代扣.已保存代扣
  }
  if (data.min_balance !== undefined) {
    代理代扣.min_balance = Number(data.min_balance) || 0
  }
}
// 开关只修改本地草稿，显式保存后才更新服务端；失败时恢复到已保存状态。
const 保存代理代扣 = function () {
  if (代理代扣.保存中 || 代理代扣.加载中 || !代理代扣有改动.value) return
  代理代扣.保存中 = true
  return post('/point_card/settings', { point_card_auto_deduct: 代理代扣.point_card_auto_deduct })
    .then((res) => {
      if (!res.data?.state) throw new Error(res.data?.msg || '保存点卡代扣设置失败')
      同步代理代扣设置(res.data)
      ElMessage.success('点卡代扣设置已保存')
    })
    .catch((error) => {
      代理代扣.point_card_auto_deduct = 代理代扣.已保存代扣
      显示错误(error)
    })
    .finally(() => {
      代理代扣.保存中 = false
    })
}
const 查询软件 = function (force = false) {
  代理代扣.加载中 = 是代理账号.value
  return stores.查询软件列表(force)
    .then(() => {
      if (是代理账号.value) {
        同步代理代扣设置(stores.账号信息)
      }
    })
    .catch(显示错误)
    .finally(() => {
      代理代扣.加载中 = false
    })
}
const 查询代理 = function (force = false) {
  if (是代理账号.value) return Promise.resolve()
  return stores.查询代理列表(force)
}
// 两种身份分别使用自己的只读列表接口，一次返回全部有权查看的软件方案。
const 查询全部价格 = function () {
  点卡方案.加载中 = true
  return post('/point_card/price/list', {})
    .then((res) => {
      if (!res.data?.state) throw new Error(res.data?.msg || '查询点卡计费方案失败')
      点卡方案.rows = res.data.data || []
      点卡方案.已加载 = true
    })
    .catch((error) => {
      点卡方案.已加载 = false
      显示错误(error)
    })
    .finally(() => { 点卡方案.加载中 = false })
}
const 刷新页面 = async function () {
  if (加载中.value || 代理代扣.保存中) return
  加载中.value = true
  try {
    await Promise.all([查询软件(true), 是代理账号.value ? 查询代理价格(true) : 查询代理(true), 查询全部价格()])
  } catch (error) {
    显示错误(error)
  } finally {
    加载中.value = false
  }
}
const 查询代理价格 = function (force = false) {
  if (代理价格加载中.value) return Promise.resolve()
  if (!是代理账号.value) return Promise.resolve()
  代理价格加载中.value = true
  return stores.查询代理时长价格列表(force)
    .catch(显示错误)
    .finally(() => {
      代理价格加载中.value = false
    })
}
const 打开软件编辑 = function () {
  Object.assign(软件框, {
    显示: true,
    id: 0,
    software: '',
    bulletin: '',
    default_period_minutes: 60,
    heartbeat_interval_minutes: 5,
    online_grace_minutes: 60,
    point_card_reuse_enabled: true,
    pause_deduct_minutes: 0
  })
}
const 编辑软件 = function (row) {
  Object.assign(软件框, {
    显示: true,
    id: row.ID,
    software: row.Software,
    bulletin: row.Bulletin || '',
    default_period_minutes: Number(row.default_period_minutes || 60),
    heartbeat_interval_minutes: 心跳秒转分钟(row.heartbeat_interval_seconds || 300),
    online_grace_minutes: Number(row.online_grace_minutes || 60),
    point_card_reuse_enabled: Boolean(row.point_card_reuse_enabled),
    pause_deduct_minutes: Number(row.pause_deduct_minutes || 0)
  })
}
const 保存软件 = function () {
  if (软件框.加载中) return
  const heartbeatSeconds = Math.round(软件框.heartbeat_interval_minutes * 60)
  if (!软件框.software.trim()) {
    ElMessage.warning('请输入软件名称')
    return
  }
  if (!点卡计费周期有效(软件框.default_period_minutes)) {
    ElMessage.warning('默认授权时长必须在15至43200分钟之间')
    return
  }
  if (
    !Number.isInteger(软件框.online_grace_minutes) ||
    (软件框.online_grace_minutes !== 0 && 软件框.online_grace_minutes < 最小自动离线时间分钟) ||
    软件框.online_grace_minutes > 最大自动离线时间分钟
  ) {
    ElMessage.warning('自动离线时间必须为0或5至4320分钟，0表示默认60分钟')
    return
  }
  if (
    !Number.isFinite(软件框.heartbeat_interval_minutes) ||
    heartbeatSeconds < 1 ||
    heartbeatSeconds > 86400
  ) {
    ElMessage.warning('心跳间隔必须在1秒至1440分钟之间')
    return
  }
  if (
    !Number.isInteger(软件框.pause_deduct_minutes) ||
    软件框.pause_deduct_minutes < 0 ||
    软件框.pause_deduct_minutes > 最大时长分钟
  ) {
    ElMessage.warning(`暂停扣除时长必须为0至${最大时长分钟 / 1440}天`)
    return
  }
  软件框.加载中 = true
  const url = 软件框.id ? '/user_modify_bulletin' : '/user_add_soft'
  post(url, {
    id: 软件框.id,
    software: 软件框.software,
    bulletin: 软件框.bulletin,
    default_period_minutes: 软件框.default_period_minutes,
    heartbeat_interval_seconds: heartbeatSeconds,
    online_grace_minutes: 软件框.online_grace_minutes,
    point_card_reuse_enabled: 软件框.point_card_reuse_enabled,
    pause_deduct_minutes: 软件框.pause_deduct_minutes
  })
    .then((res) => {
      if (!res.data?.state) throw new Error(res.data?.msg || '保存软件失败')
      ElMessage.success('保存成功')
      软件框.显示 = false
      return Promise.all([查询软件(true), 查询全部价格()])
    })
    .catch(显示错误)
    .finally(() => {
      软件框.加载中 = false
    })
}
const 删除软件 = function (row) {
  ElMessageBox.confirm(`删除软件“${row.Software}”会同时删除其点卡、时长卡和设备会话，流水仅保留最近30天。继续？`, '确认删除', {
    type: 'warning'
  })
    .then(() => post('/user_del_soft', { id: row.ID }))
    .then((res) => {
      if (!res.data?.state) throw new Error(res.data?.msg || '删除软件失败')
      ElMessage.success('删除成功')
      return Promise.all([查询软件(true), 查询全部价格()])
    })
    .catch((error) => {
      if (error !== 'cancel' && error !== 'close') 显示错误(error)
    })
}

const 打开价格 = function (row) {
  Object.assign(价格框, {
    显示: true,
    software: row.ID,
    softwareName: row.Software,
    onlineGraceMinutes: Number(row.online_grace_minutes || 60),
    rows: []
  })
  查询价格()
}
const 查询价格 = function () {
  const software = 价格框.software
  价格框.加载中 = true
  return post('/point_card/price/list', { software: 价格框.software })
    .then((res) => {
      if (!res.data?.state) throw new Error(res.data?.msg || '查询点卡计费方案失败')
      if (价格框.software === software) {
        价格框.rows = (res.data.data || []).map((item) => ({ ...item, 原默认: Boolean(item.is_default), 保存中: false }))
      }
    })
    .catch(显示错误)
    .finally(() => { if (价格框.software === software) 价格框.加载中 = false })
}
const 新增价格 = function () {
  if (价格框.加载中 || 价格框.保存中 || 价格框.rows.some((item) => !item.id)) return
  const usedPeriods = new Set(价格框.rows.map((item) => Number(item.period_minutes)))
  价格框.rows.push({
    id: 0,
    software: 价格框.software,
    period_minutes: [60, 120, 1440, 15, 30].find((minutes) => !usedPeriods.has(minutes)) ?? 60,
    cost: 1,
    enabled: true,
    is_default: !价格框.rows.length,
    原默认: false,
    保存中: false
  })
}
const 保存价格 = function (row) {
  if (价格框.保存中 || row.保存中) return
  if (!点卡计费周期有效(row.period_minutes)) {
    ElMessage.warning('授权时长必须在15至43200分钟之间')
    return
  }
  if (!Number.isInteger(row.cost) || row.cost < 1 || row.cost > 1000000000) {
    ElMessage.warning('扣点数必须在1至1000000000之间')
    return
  }
  if (!row.id && 价格框.rows.some((item) => item.id && Number(item.period_minutes) === Number(row.period_minutes))) {
    ElMessage.warning('该授权时长已存在，请直接修改原方案')
    return
  }
  if (row.is_default && !row.enabled) {
    ElMessage.warning('默认方案必须启用')
    return
  }
  row.保存中 = true
  价格框.保存中 = true
  const software = 价格框.software
  post('/point_card/price/save', {
    software,
    period_minutes: row.period_minutes,
    cost: row.cost,
    enabled: row.enabled,
    is_default: row.is_default
  })
    .then((res) => {
      if (!res.data?.state) throw new Error(res.data?.msg || '保存价格失败')
      ElMessage.success('保存成功')
      if (价格框.software === software) {
        const saved = res.data.data
        if (saved.is_default) {
          // 后端切换默认方案会同时清除其他行的默认标记，行内状态随之同步。
          价格框.rows.forEach((item) => {
            if (item !== row) item.is_default = item.原默认 = false
          })
        }
        Object.assign(row, saved, { 原默认: Boolean(saved.is_default) })
      }
      return Promise.all([查询软件(true), 查询全部价格()])
    })
    .catch(显示错误)
    .finally(() => {
      row.保存中 = false
      价格框.保存中 = false
    })
}
const 删除价格 = function (row) {
  if (价格框.保存中) return
  if (!row.id) {
    价格框.rows = 价格框.rows.filter((item) => item !== row)
    return
  }
  ElMessageBox.confirm('删除后使用该授权时长的登录会被拒绝，确定删除？', '确认删除', { type: 'warning' })
    .then(() => post('/point_card/price/delete', { id: row.id }))
    .then((res) => {
      if (!res.data?.state) throw new Error(res.data?.msg || '删除失败')
      价格框.rows = 价格框.rows.filter((item) => item !== row)
      return 查询全部价格()
    })
    .catch((error) => {
      if (error !== 'cancel' && error !== 'close') 显示错误(error)
    })
}

const 创建代理 = function () {
  const passwordBytes = new TextEncoder().encode(代理框.password || '').length
  if (!/^[A-Za-z0-9_]{3,32}$/.test(代理框.name)) {
    ElMessage.warning('渠道合伙人账号只能使用3至32位字母、数字或下划线')
    return
  }
  if (passwordBytes < 6 || passwordBytes > 72) {
    ElMessage.warning('密码长度必须为6至72个字节')
    return
  }
  post('/创建代理账号', { agent_name: 代理框.name, agent_password: 代理框.password })
    .then((res) => {
      if (!res.data?.state) throw new Error(res.data?.msg || '创建失败')
      ElMessage.success('创建成功')
      代理框.显示 = false
      Object.assign(代理框, { name: '', password: '' })
      查询代理(true)
    })
    .catch(显示错误)
}
const 打开代理创建 = function () {
  Object.assign(代理框, { 显示: true, name: '', password: '' })
}
const 编辑代理 = function (row) {
  let prices = {}
  try {
    prices = typeof row.prices === 'string' ? JSON.parse(row.prices || '{}') : row.prices || {}
  } catch {
    prices = {}
  }
  const normalized = {}
  软件列表.value.forEach((item) => {
    const value = Number(prices[item.ID] ?? prices[String(item.ID)] ?? 0)
    normalized[item.ID] = Number.isFinite(value) && value >= 0 ? value : 0
  })
  Object.assign(代理编辑框, {
    显示: true,
    id: row.id,
    name: row.name,
    balance: row.balance,
    prices: normalized,
    password: '',
    min_balance: Number(row.min_balance) || 0
  })
}

// 打开时长卡价格设置时先选择一个软件，并从服务端读取该代理的完整锚点
// 集合。空集合也有明确含义：该代理暂时不能为这个软件生成时长卡。
const 打开代理时长价格 = function (row) {
  Object.assign(代理时长价格框, {
    显示: true,
    加载中: false,
    保存中: false,
    agent_id: Number(row.id),
    name: row.name || '',
    software: 软件列表.value[0]?.ID || 0,
    rows: []
  })
  查询代理时长价格()
}
const 查询代理时长价格 = function () {
  if (!代理时长价格框.agent_id || !代理时长价格框.software) {
    代理时长价格框.rows = []
    return Promise.resolve()
  }
  代理时长价格框.加载中 = true
  return post('/duration_card/agent_price/list', {
    agent_id: 代理时长价格框.agent_id,
    software: 代理时长价格框.software
  })
    .then((res) => {
      if (!res.data?.state) throw new Error(res.data?.msg || '查询时长卡价格失败')
      代理时长价格框.rows = (res.data.data || []).map((item) => ({
        id: Number(item.id || 0),
        duration_minutes: Number(item.duration_minutes || 0),
        原始时长分钟: Number(item.duration_minutes || 0),
        price: Number(item.price || 0),
        enabled: Boolean(item.enabled)
      }))
    })
    .catch(显示错误)
    .finally(() => {
      代理时长价格框.加载中 = false
    })
}
const 打开代理时长价格总览 = function () {
  Object.assign(代理时长价格总览框, { 显示: true, 加载中: false, agent_id: 0, software: 0, rows: [] })
  查询代理时长价格总览()
}
const 查询代理时长价格总览 = async function () {
  if (代理时长价格总览框.加载中) return
  const agents = 代理列表.value.filter((item) => !代理时长价格总览框.agent_id || Number(item.id) === Number(代理时长价格总览框.agent_id))
  const softwares = 软件列表.value.filter((item) => !代理时长价格总览框.software || Number(item.ID) === Number(代理时长价格总览框.software))
  代理时长价格总览框.加载中 = true
  try {
    const rows = []
    // 接口支持一次返回该代理全部软件价格；最多并发四个代理，避免代理数×软件数的请求风暴。
    for (let start = 0; start < agents.length; start += 4) {
      const groups = await Promise.all(agents.slice(start, start + 4).map(async (agent) => {
          const res = await post('/duration_card/agent_price/list', { agent_id: agent.id, software: 0 })
          if (!res.data?.state) throw new Error(res.data?.msg || '查询时长卡价格总览失败')
          return softwares.map((software) => ({
            key: `${agent.id}-${software.ID}`,
            agent_id: Number(agent.id),
            agent_name: agent.name,
            software: Number(software.ID),
            software_name: software.Software,
            保存中: false,
            prices: (res.data.data || []).filter((item) => Number(item.software) === Number(software.ID)).map((item) => ({
              key: `price-${item.id}`,
              id: Number(item.id || 0),
              duration_minutes: Number(item.duration_minutes || 0),
              price: Number(item.price || 0),
              enabled: Boolean(item.enabled)
            }))
          }))
      }))
      rows.push(...groups.flat())
    }
    代理时长价格总览框.rows = rows
  } catch (error) {
    显示错误(error)
  } finally {
    代理时长价格总览框.加载中 = false
  }
}
const 新增总览锚点 = function (row) {
  if (row.保存中 || row.prices.length >= 50) return
  row.prices.push({
    key: `new-${Date.now()}-${row.prices.length}`,
    id: 0,
    duration_minutes: 1440,
    price: 1,
    enabled: true
  })
}
const 删除总览锚点 = function (row, index) {
  if (row.保存中) return
  ElMessageBox.confirm(`确定删除${时长卡时长文本(row.prices[index].duration_minutes)}价格锚点？`, '确认删除', { type: 'warning' })
    .then(() => row.prices.splice(index, 1))
    .catch((error) => {
      if (error !== 'cancel' && error !== 'close') 显示错误(error)
    })
}
const 保存总览价格 = function (row) {
  if (row.保存中) return
  const seen = new Set()
  for (const price of row.prices) {
    if (!Number.isInteger(price.duration_minutes) || price.duration_minutes < 最小时长分钟 || price.duration_minutes > 最大时长分钟) {
      ElMessage.warning('卡面时长必须为5分钟至36500天的整数')
      return
    }
    if (seen.has(price.duration_minutes)) {
      ElMessage.warning('卡面时长不能重复')
      return
    }
    if (!Number.isFinite(price.price) || price.price <= 0 || price.price > 最大代理价格 || Math.abs(price.price * 100 - Math.round(price.price * 100)) > 1e-8) {
      ElMessage.warning('代理价格必须大于0且最多保留两位小数')
      return
    }
    seen.add(price.duration_minutes)
  }
  row.保存中 = true
  post('/duration_card/agent_price/save', {
    agent_id: row.agent_id,
    software: row.software,
    prices: row.prices.map((price) => ({ duration_minutes: price.duration_minutes, price: Number(price.price), enabled: Boolean(price.enabled) }))
  })
    .then((res) => {
      if (!res.data?.state) throw new Error(res.data?.msg || '保存时长卡价格失败')
      ElMessage.success('保存成功')
    })
    .catch(显示错误)
    .finally(() => {
      row.保存中 = false
    })
}
const 新增代理时长价格 = function () {
  代理时长价格框.rows.push({
    id: 0,
    duration_minutes: 1440,
    原始时长分钟: 0,
    price: 1,
    enabled: true
  })
}
const 删除代理时长价格 = function (index, row) {
  if (!row.id) {
    代理时长价格框.rows.splice(index, 1)
    return
  }
  ElMessageBox.confirm(`确定删除${时长卡时长文本(row.duration_minutes)}价格锚点？`, '确认删除', { type: 'warning' })
    .then(() => post('/duration_card/agent_price/delete', {
      agent_id: 代理时长价格框.agent_id,
      software: 代理时长价格框.software,
      // 已保存锚点允许先在表格中编辑时长；删除时必须用数据库中的原始
      // 时长，避免把编辑后的值误当成删除条件。
      duration_minutes: row.原始时长分钟 || row.duration_minutes
    }))
    .then((res) => {
      if (!res.data?.state) throw new Error(res.data?.msg || '删除时长卡价格失败')
      ElMessage.success(res.data.msg || '删除成功')
      // 只移除已删除行，不重新查询整组，保留用户尚未保存的其他编辑。
      代理时长价格框.rows.splice(index, 1)
    })
    .catch((error) => {
      if (error !== 'cancel' && error !== 'close') 显示错误(error)
    })
}
const 保存代理时长价格 = function () {
  if (代理时长价格框.保存中 || 代理时长价格框.加载中) return
  if (!代理时长价格框.agent_id || !代理时长价格框.software) {
    ElMessage.warning('请选择软件')
    return
  }
  const seen = new Set()
  for (const row of 代理时长价格框.rows) {
    if (!Number.isInteger(row.duration_minutes) || row.duration_minutes < 最小时长分钟 || row.duration_minutes > 最大时长分钟) {
      ElMessage.warning('卡面时长必须为5分钟至36500天的整数')
      return
    }
    if (seen.has(row.duration_minutes)) {
      ElMessage.warning('卡面时长不能重复')
      return
    }
    const price = Number(row.price)
    const scaledPrice = price * 100
    if (!Number.isFinite(price) || price <= 0 || price > 最大代理价格 || Math.abs(scaledPrice - Math.round(scaledPrice)) > 1e-8) {
      ElMessage.warning('代理价格必须大于0且最多保留两位小数')
      return
    }
    seen.add(row.duration_minutes)
  }
  代理时长价格框.保存中 = true
  post('/duration_card/agent_price/save', {
    agent_id: 代理时长价格框.agent_id,
    software: 代理时长价格框.software,
    // 空数组明确表示清空本软件的全部价格锚点。
    prices: 代理时长价格框.rows.map((row) => ({
      duration_minutes: row.duration_minutes,
      price: Number(row.price),
      enabled: Boolean(row.enabled)
    }))
  })
    .then((res) => {
      if (!res.data?.state) throw new Error(res.data?.msg || '保存时长卡价格失败')
      ElMessage.success(res.data.msg || '保存成功')
      return 查询代理时长价格()
    })
    .catch(显示错误)
    .finally(() => {
      代理时长价格框.保存中 = false
    })
}
const 保存代理 = function () {
  if (代理编辑框.password) {
    const passwordBytes = new TextEncoder().encode(代理编辑框.password).length
    if (passwordBytes < 6 || passwordBytes > 72) {
      ElMessage.warning('新密码长度必须为6至72个字节')
      return
    }
  }
  if (
    !Number.isInteger(代理编辑框.min_balance) ||
    代理编辑框.min_balance < -最大代理余额下限绝对值 ||
    代理编辑框.min_balance > 最大代理余额下限绝对值
  ) {
    ElMessage.warning(`余额下限必须为-${最大代理余额下限绝对值}至${最大代理余额下限绝对值}点`)
    return
  }
  // 价格为 0 的软件不写入映射，缺少映射即表示代理无权为该软件发卡。
  const prices = {}
  Object.keys(代理编辑框.prices).forEach((key) => {
    const value = Number(代理编辑框.prices[key] || 0)
    if (Number.isFinite(value) && value > 0) prices[key] = value
  })
  post('/设置代理账号', {
    data: {
      id: 代理编辑框.id,
      password: 代理编辑框.password,
      prices: JSON.stringify(prices),
      min_balance: 代理编辑框.min_balance
    }
  })
    .then((res) => {
      if (!res.data?.state) throw new Error(res.data?.msg || '保存渠道合伙人失败')
      ElMessage.success('保存成功')
      代理编辑框.显示 = false
      查询代理(true)
    })
    .catch(显示错误)
}
const 打开代理充值 = function (row) {
  Object.assign(代理充值框, { 显示: true, id: row.id, name: row.name, amount: 0, note: '' })
}
const 打开代理日志 = function (row) {
  Object.assign(代理日志框, { 显示: true, 加载中: false, id: row.id, name: row.name, content: '' })
  查询代理日志()
}
const 查询代理日志 = async function () {
  if (!代理日志框.id || 代理日志框.加载中) return
  const id = 代理日志框.id
  代理日志框.加载中 = true
  try {
    const response = await post('/查询代理账号日志', { id })
    if (代理日志框.id !== id) return
    if (typeof response.data === 'string') {
      代理日志框.content = 日志倒序(response.data)
    } else if (!response.data?.state) {
      throw new Error(response.data?.msg || '查询渠道合伙人日志失败')
    } else {
      代理日志框.content = 日志倒序(response.data.data)
    }
  } catch (error) {
    if (代理日志框.id === id) 显示错误(error)
  } finally {
    if (代理日志框.id === id) 代理日志框.加载中 = false
  }
}
const 代理充值 = function () {
  if (代理充值框.保存中) return
  if (!Number.isSafeInteger(代理充值框.amount) || 代理充值框.amount === 0) {
    ElMessage.warning('请输入非零的整数变更点数')
    return
  }
  const request = { id: 代理充值框.id, amount: 代理充值框.amount, note: 代理充值框.note }
  代理充值框.保存中 = true
  ElMessageBox.confirm(`确定调整 ${request.amount} 点？`, '确认余额调整', {
    type: 'warning'
  })
    .then(() => post('/代理账号充值', request))
    .then((res) => {
      if (!res.data?.state) throw new Error(res.data?.msg || '余额调整失败')
      ElMessage.success('余额调整成功')
      代理充值框.显示 = false
      查询代理(true)
    })
    .catch((error) => {
      if (error !== 'cancel' && error !== 'close') 显示错误(error)
    })
    .finally(() => { 代理充值框.保存中 = false })
}
const 删除代理 = function (row) {
  ElMessageBox.confirm(`确定删除渠道合伙人“${row.name}”？已生成的点卡和流水仅保留最近30天。`, '确认删除', {
    type: 'warning'
  })
    .then(() => post('/删除代理账号', { id: row.id }))
    .then((res) => {
      if (!res.data?.state) throw new Error(res.data?.msg || '删除失败')
      ElMessage.success('删除成功')
      查询代理(true)
    })
    .catch((error) => {
      if (error !== 'cancel' && error !== 'close') 显示错误(error)
    })
}

onMounted(() => {
  if (是代理账号.value) {
    Promise.all([查询软件(), 查询代理价格(), 查询全部价格()]).catch(() => {})
  } else {
    Promise.all([查询软件(), 查询代理(), 查询全部价格()]).catch(() => {})
  }
})
</script>

<style scoped>
.页面 {
  padding: 16px;
  color: #e6eaf2;
}
.页面标题行,
.子标题行,
.价格标题 {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
}
h2,
h3 {
  margin: 0 0 6px;
}
.说明 {
  margin: 0 0 12px;
  color: #aeb6c3;
  font-size: 13px;
}
.代理区 {
  margin-top: 24px;
}
.方案列表 {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
.代理日志内容 {
  min-height: 240px;
  max-height: 60vh;
  overflow: auto;
  margin-top: 12px;
}
.代理日志内容 pre {
  margin: 0;
  white-space: pre-wrap;
  word-break: break-word;
  color: #cbd3df;
  line-height: 1.6;
}
.时长价格分组 {
  display: grid;
  gap: 12px;
}
.时长价格组 {
  padding: 12px;
  border: 1px solid var(--el-border-color);
  border-radius: 6px;
}
.时长价格组标题 {
  margin-bottom: 10px;
  font-weight: 600;
}
.代理设置区 {
  display: grid;
  gap: 14px;
  margin-top: 16px;
}
.代扣设置栏 {
  padding: 4px 2px;
}
.代扣设置主行 {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px 14px;
}
.代扣设置名称 {
  font-size: 14px;
  font-weight: 600;
}
.代理设置标题 {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}
.设置说明 {
  margin: 0;
  color: #aeb6c3;
  font-size: 13px;
}
.代理设置状态 {
  display: flex;
  flex-wrap: wrap;
  gap: 8px 24px;
  margin-top: 12px;
  color: #aeb6c3;
  font-size: 13px;
}
.账户详情切换 {
  margin-left: auto;
}
.价格标题 {
  margin-bottom: 12px;
}
.方案时长输入,
.方案扣点输入 {
  width: 120px;
}
.方案提醒 {
  margin-top: 4px;
  font-size: 12px;
}
.代理价格标题 {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  margin-bottom: 12px;
}
.代理价格提示 {
  margin-bottom: 12px;
}
.价格单位 {
  margin-left: 8px;
  color: #9099a8;
  font-size: 12px;
}
.价格说明 {
  margin-left: 10px;
  color: #9099a8;
  font-size: 12px;
}
.字段说明 {
  margin-left: 10px;
  color: #9099a8;
  font-size: 12px;
}
.风险提醒 {
  color: #d97706;
}
</style>

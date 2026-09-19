import { useEffect, useRef, useCallback } from 'react';
import * as Blockly from 'blockly';
import { javascriptGenerator, Order } from 'blockly/javascript';

// ── Custom Block Definitions ──────────────────────────────────────────────────

function defineBlocks() {
  // Block: IF domain matches category → BLOCK
  Blockly.Blocks['dns_if_category'] = {
    init() {
      this.appendValueInput('CATEGORY')
        .setCheck('Category')
        .appendField('If domain is in category');
      this.appendStatementInput('ACTION')
        .appendField('then');
      this.setPreviousStatement(true, null);
      this.setNextStatement(true, null);
      this.setColour(210);
      this.setTooltip('Block or allow domains based on category');
    },
  };
  javascriptGenerator.forBlock['dns_if_category'] = (block) => {
    const category = javascriptGenerator.valueToCode(block, 'CATEGORY', Order.ATOMIC) || '""';
    const action = javascriptGenerator.statementToCode(block, 'ACTION');
    return `if (matchesCategory(${category})) {\n${action}}\n`;
  };

  // Block: Category value
  Blockly.Blocks['dns_category'] = {
    init() {
      this.appendDummyInput()
        .appendField('category')
        .appendField(new Blockly.FieldDropdown([
          ['Adult Content', 'ADULT'],
          ['Social Media', 'SOCIAL'],
          ['Gaming', 'GAMING'],
          ['Gambling', 'GAMBLING'],
          ['Streaming', 'STREAMING'],
          ['Ads & Trackers', 'ADS'],
          ['Violence', 'VIOLENCE'],
          ['Malware', 'MALWARE'],
        ]), 'CATEGORY');
      this.setOutput(true, 'Category');
      this.setColour(160);
    },
  };
  javascriptGenerator.forBlock['dns_category'] = (block) => {
    const cat = block.getFieldValue('CATEGORY');
    return [`"${cat}"`, Order.ATOMIC];
  };

  // Block: Block domain
  Blockly.Blocks['dns_block'] = {
    init() {
      this.appendDummyInput().appendField('🚫 Block');
      this.setPreviousStatement(true, null);
      this.setNextStatement(true, null);
      this.setColour(0);
    },
  };
  javascriptGenerator.forBlock['dns_block'] = () => '  action = "BLOCK";\n';

  // Block: Allow domain
  Blockly.Blocks['dns_allow'] = {
    init() {
      this.appendDummyInput().appendField('✅ Allow');
      this.setPreviousStatement(true, null);
      this.setNextStatement(true, null);
      this.setColour(120);
    },
  };
  javascriptGenerator.forBlock['dns_allow'] = () => '  action = "ALLOW";\n';

  // Block: Block specific domain
  Blockly.Blocks['dns_block_domain'] = {
    init() {
      this.appendDummyInput()
        .appendField('🚫 Block domain')
        .appendField(new Blockly.FieldTextInput('example.com'), 'DOMAIN');
      this.setPreviousStatement(true, null);
      this.setNextStatement(true, null);
      this.setColour(0);
    },
  };
  javascriptGenerator.forBlock['dns_block_domain'] = (block) => {
    const domain = block.getFieldValue('DOMAIN');
    return `  blockedDomains.push("${domain}");\n`;
  };

  // Block: Allow specific domain
  Blockly.Blocks['dns_allow_domain'] = {
    init() {
      this.appendDummyInput()
        .appendField('✅ Allow domain')
        .appendField(new Blockly.FieldTextInput('example.com'), 'DOMAIN');
      this.setPreviousStatement(true, null);
      this.setNextStatement(true, null);
      this.setColour(120);
    },
  };
  javascriptGenerator.forBlock['dns_allow_domain'] = (block) => {
    const domain = block.getFieldValue('DOMAIN');
    return `  allowedDomains.push("${domain}");\n`;
  };

  // Block: Time-based rule
  Blockly.Blocks['dns_time_rule'] = {
    init() {
      this.appendDummyInput()
        .appendField('⏰ Between')
        .appendField(new Blockly.FieldNumber(8, 0, 23), 'FROM')
        .appendField(':00 and')
        .appendField(new Blockly.FieldNumber(22, 0, 23), 'TO')
        .appendField(':00');
      this.appendStatementInput('ACTION').appendField('then');
      this.setPreviousStatement(true, null);
      this.setNextStatement(true, null);
      this.setColour(290);
    },
  };
  javascriptGenerator.forBlock['dns_time_rule'] = (block) => {
    const from = block.getFieldValue('FROM');
    const to = block.getFieldValue('TO');
    const action = javascriptGenerator.statementToCode(block, 'ACTION');
    return `if (isTimeBetween(${from}, ${to})) {\n${action}}\n`;
  };

  // Block: Safe Search
  Blockly.Blocks['dns_safe_search'] = {
    init() {
      this.appendDummyInput().appendField('🔍 Enable Safe Search');
      this.setPreviousStatement(true, null);
      this.setNextStatement(true, null);
      this.setColour(45);
    },
  };
  javascriptGenerator.forBlock['dns_safe_search'] = () => '  flags.safeSearch = true;\n';

  // ── IF / ELSE ────────────────────────────────────────────────────────────────

  Blockly.Blocks['dns_if_else_category'] = {
    init() {
      this.appendValueInput('CATEGORY')
        .setCheck('Category')
        .appendField('🔀 If category is');
      this.appendStatementInput('IF_ACTION').appendField('then');
      this.appendStatementInput('ELSE_ACTION').appendField('else');
      this.setPreviousStatement(true, null);
      this.setNextStatement(true, null);
      this.setColour(210);
      this.setTooltip('If category matches, do one thing. Otherwise, do another.');
    },
  };
  javascriptGenerator.forBlock['dns_if_else_category'] = (block) => {
    const category = javascriptGenerator.valueToCode(block, 'CATEGORY', Order.ATOMIC) || '""';
    const ifAction = javascriptGenerator.statementToCode(block, 'IF_ACTION');
    const elseAction = javascriptGenerator.statementToCode(block, 'ELSE_ACTION');
    return `if (matchesCategory(${category})) {\n${ifAction}} else {\n${elseAction}}\n`;
  };

  // ── SWITCH / CASE ────────────────────────────────────────────────────────────

  Blockly.Blocks['dns_switch_category'] = {
    init() {
      this.appendDummyInput().appendField('🔀 Switch on category');
      this.appendValueInput('CASE_ADULT').setCheck('String').appendField('Adult →');
      this.appendValueInput('CASE_SOCIAL').setCheck('String').appendField('Social Media →');
      this.appendValueInput('CASE_GAMING').setCheck('String').appendField('Gaming →');
      this.appendValueInput('CASE_GAMBLING').setCheck('String').appendField('Gambling →');
      this.setPreviousStatement(true, null);
      this.setNextStatement(true, null);
      this.setColour(190);
      this.setTooltip('Apply different actions based on domain category');
    },
  };
  javascriptGenerator.forBlock['dns_switch_category'] = (block) => {
    const adult = javascriptGenerator.valueToCode(block, 'CASE_ADULT', Order.ATOMIC);
    const social = javascriptGenerator.valueToCode(block, 'CASE_SOCIAL', Order.ATOMIC);
    const gaming = javascriptGenerator.valueToCode(block, 'CASE_GAMING', Order.ATOMIC);
    const gambling = javascriptGenerator.valueToCode(block, 'CASE_GAMBLING', Order.ATOMIC);
    return `switch(getCategory()) {\n  case "ADULT": ${adult || 'action="BLOCK"'}; break;\n  case "SOCIAL": ${social || 'action="ALLOW"'}; break;\n  case "GAMING": ${gaming || 'action="ALLOW"'}; break;\n  case "GAMBLING": ${gambling || 'action="BLOCK"'}; break;\n}\n`;
  };

  // Block: action values for switch
  Blockly.Blocks['dns_action_value'] = {
    init() {
      this.appendDummyInput()
        .appendField(new Blockly.FieldDropdown([
          ['Block', 'action="BLOCK"'],
          ['Allow', 'action="ALLOW"'],
        ]), 'ACTION');
      this.setOutput(true, 'String');
      this.setColour(60);
    },
  };
  javascriptGenerator.forBlock['dns_action_value'] = (block) => {
    const action = block.getFieldValue('ACTION');
    return [action, Order.ATOMIC];
  };

  // ── TRY / CATCH ─────────────────────────────────────────────────────────────

  Blockly.Blocks['dns_try_catch'] = {
    init() {
      this.appendStatementInput('TRY').appendField('🧪 Try');
      this.appendStatementInput('CATCH').appendField('⚠️ If error, fallback');
      this.setPreviousStatement(true, null);
      this.setNextStatement(true, null);
      this.setColour(30);
      this.setTooltip('Try a rule. If it fails (e.g., unknown category), run the fallback.');
    },
  };
  javascriptGenerator.forBlock['dns_try_catch'] = (block) => {
    const tryCode = javascriptGenerator.statementToCode(block, 'TRY');
    const catchCode = javascriptGenerator.statementToCode(block, 'CATCH');
    return `try {\n${tryCode}} catch(e) {\n${catchCode}}\n`;
  };

  // ── DAY OF WEEK ─────────────────────────────────────────────────────────────

  Blockly.Blocks['dns_day_rule'] = {
    init() {
      this.appendDummyInput()
        .appendField('📅 On day')
        .appendField(new Blockly.FieldDropdown([
          ['Monday', '1'],
          ['Tuesday', '2'],
          ['Wednesday', '3'],
          ['Thursday', '4'],
          ['Friday', '5'],
          ['Saturday', '6'],
          ['Sunday', '0'],
        ]), 'DAY');
      this.appendStatementInput('ACTION').appendField('then');
      this.setPreviousStatement(true, null);
      this.setNextStatement(true, null);
      this.setColour(65);
      this.setTooltip('Apply rule only on a specific day of the week');
    },
  };
  javascriptGenerator.forBlock['dns_day_rule'] = (block) => {
    const day = block.getFieldValue('DAY');
    const action = javascriptGenerator.statementToCode(block, 'ACTION');
    return `if (new Date().getDay() === ${day}) {\n${action}}\n`;
  };

  Blockly.Blocks['dns_weekend_rule'] = {
    init() {
      this.appendDummyInput().appendField('📅 On Weekends');
      this.appendStatementInput('ACTION').appendField('then');
      this.setPreviousStatement(true, null);
      this.setNextStatement(true, null);
      this.setColour(65);
    },
  };
  javascriptGenerator.forBlock['dns_weekend_rule'] = (block) => {
    const action = javascriptGenerator.statementToCode(block, 'ACTION');
    return `if ([0,6].includes(new Date().getDay())) {\n${action}}\n`;
  };

  Blockly.Blocks['dns_weekday_rule'] = {
    init() {
      this.appendDummyInput().appendField('📅 On Weekdays (Mon-Fri)');
      this.appendStatementInput('ACTION').appendField('then');
      this.setPreviousStatement(true, null);
      this.setNextStatement(true, null);
      this.setColour(65);
    },
  };
  javascriptGenerator.forBlock['dns_weekday_rule'] = (block) => {
    const action = javascriptGenerator.statementToCode(block, 'ACTION');
    return `if (new Date().getDay() >= 1 && new Date().getDay() <= 5) {\n${action}}\n`;
  };

  // ── TIME RANGE (exact, with minutes) ─────────────────────────────────────────

  Blockly.Blocks['dns_time_range_exact'] = {
    init() {
      this.appendDummyInput()
        .appendField('⏰ From')
        .appendField(new Blockly.FieldNumber(8, 0, 23), 'FROM_H')
        .appendField(':')
        .appendField(new Blockly.FieldNumber(0, 0, 59), 'FROM_M')
        .appendField('to')
        .appendField(new Blockly.FieldNumber(22, 0, 23), 'TO_H')
        .appendField(':')
        .appendField(new Blockly.FieldNumber(0, 0, 59), 'TO_M');
      this.appendStatementInput('ACTION').appendField('then');
      this.setPreviousStatement(true, null);
      this.setNextStatement(true, null);
      this.setColour(290);
      this.setTooltip('Apply rule within exact time range (hours:minutes)');
    },
  };
  javascriptGenerator.forBlock['dns_time_range_exact'] = (block) => {
    const fh = block.getFieldValue('FROM_H');
    const fm = block.getFieldValue('FROM_M');
    const th = block.getFieldValue('TO_H');
    const tm = block.getFieldValue('TO_M');
    const action = javascriptGenerator.statementToCode(block, 'ACTION');
    return `{\n  const _now = new Date();\n  const _mins = _now.getHours()*60+_now.getMinutes();\n  if (_mins >= ${+fh*60 + +fm} && _mins <= ${+th*60 + +tm}) {\n${action}  }\n}\n`;
  };

  // ── DAY + TIME COMBINED ──────────────────────────────────────────────────────

  Blockly.Blocks['dns_day_time_combined'] = {
    init() {
      this.appendDummyInput()
        .appendField('📅⏰')
        .appendField(new Blockly.FieldDropdown([
          ['Weekdays', 'WEEKDAY'],
          ['Weekends', 'WEEKEND'],
          ['Monday', 'MON'],
          ['Friday', 'FRI'],
          ['Saturday', 'SAT'],
          ['Sunday', 'SUN'],
        ]), 'DAY_TYPE')
        .appendField('between')
        .appendField(new Blockly.FieldNumber(8, 0, 23), 'FROM')
        .appendField('and')
        .appendField(new Blockly.FieldNumber(22, 0, 23), 'TO')
        .appendField(':00');
      this.appendStatementInput('ACTION').appendField('then');
      this.setPreviousStatement(true, null);
      this.setNextStatement(true, null);
      this.setColour(280);
      this.setTooltip('Apply rule on specific days within a time window');
    },
  };
  javascriptGenerator.forBlock['dns_day_time_combined'] = (block) => {
    const dayType = block.getFieldValue('DAY_TYPE');
    const from = block.getFieldValue('FROM');
    const to = block.getFieldValue('TO');
    const action = javascriptGenerator.statementToCode(block, 'ACTION');
    const dayChecks: Record<string, string> = {
      WEEKDAY: 'new Date().getDay()>=1&&new Date().getDay()<=5',
      WEEKEND: '[0,6].includes(new Date().getDay())',
      MON: 'new Date().getDay()===1',
      FRI: 'new Date().getDay()===5',
      SAT: 'new Date().getDay()===6',
      SUN: 'new Date().getDay()===0',
    };
    const dayCheck = dayChecks[dayType] || 'true';
    return `if ((${dayCheck}) && isTimeBetween(${from}, ${to})) {\n${action}}\n`;
  };
}


// ── Toolbox ───────────────────────────────────────────────────────────────────

const TOOLBOX = {
  kind: 'categoryToolbox',
  contents: [
    {
      kind: 'category',
      name: '🔀 Logic',
      colour: '210',
      contents: [
        { kind: 'block', type: 'dns_if_category' },
        { kind: 'block', type: 'dns_if_else_category' },
        { kind: 'block', type: 'dns_switch_category' },
        { kind: 'block', type: 'dns_try_catch' },
        { kind: 'block', type: 'dns_action_value' },
      ],
    },
    {
      kind: 'category',
      name: '⏰ Time',
      colour: '290',
      contents: [
        { kind: 'block', type: 'dns_time_rule' },
        { kind: 'block', type: 'dns_time_range_exact' },
      ],
    },
    {
      kind: 'category',
      name: '📅 Day',
      colour: '65',
      contents: [
        { kind: 'block', type: 'dns_day_rule' },
        { kind: 'block', type: 'dns_weekend_rule' },
        { kind: 'block', type: 'dns_weekday_rule' },
        { kind: 'block', type: 'dns_day_time_combined' },
      ],
    },
    {
      kind: 'category',
      name: '🚫 Block',
      colour: '0',
      contents: [
        { kind: 'block', type: 'dns_block' },
        { kind: 'block', type: 'dns_block_domain' },
      ],
    },
    {
      kind: 'category',
      name: '✅ Allow',
      colour: '120',
      contents: [
        { kind: 'block', type: 'dns_allow' },
        { kind: 'block', type: 'dns_allow_domain' },
      ],
    },
    {
      kind: 'category',
      name: '⚙️ Options',
      colour: '45',
      contents: [
        { kind: 'block', type: 'dns_category' },
        { kind: 'block', type: 'dns_safe_search' },
      ],
    },
  ],
};

// ── Component ─────────────────────────────────────────────────────────────────

interface BlocklyRule {
  blockedDomains: string[];
  allowedDomains: string[];
  categoryRules: { category: string; action: string }[];
  safeSearch: boolean;
  code: string;
}

interface ParentalBlocklyProps {
  onRulesChange: (rules: BlocklyRule) => void;
  initialXml?: string;
}

export function ParentalBlockly({ onRulesChange, initialXml }: ParentalBlocklyProps) {
  const containerRef = useRef<HTMLDivElement>(null);
  const workspaceRef = useRef<Blockly.WorkspaceSvg | null>(null);

  const extractRules = useCallback((ws: Blockly.WorkspaceSvg): BlocklyRule => {
    const code = javascriptGenerator.workspaceToCode(ws);
    // Parse generated code into structured rules
    const blockedDomains: string[] = [];
    const allowedDomains: string[] = [];

    const blockMatches = code.match(/blockedDomains\.push\("(.+?)"\)/g) || [];
    blockMatches.forEach(m => {
      const d = m.match(/"(.+?)"/)?.[1];
      if (d) blockedDomains.push(d);
    });

    const allowMatches = code.match(/allowedDomains\.push\("(.+?)"\)/g) || [];
    allowMatches.forEach(m => {
      const d = m.match(/"(.+?)"/)?.[1];
      if (d) allowedDomains.push(d);
    });

    const safeSearch = code.includes('flags.safeSearch = true');

    return { blockedDomains, allowedDomains, categoryRules: [], safeSearch, code };
  }, []);

  useEffect(() => {
    if (!containerRef.current) return;
    defineBlocks();

    const ws = Blockly.inject(containerRef.current, {
      toolbox: TOOLBOX,
      theme: {
        name: 'zcdns',
        base: Blockly.Themes.Classic,
        componentStyles: {
          workspaceBackgroundColour: '#f8fafc',
          toolboxBackgroundColour: '#1e293b',
          toolboxForegroundColour: '#f8fafc',
          flyoutBackgroundColour: '#0f172a',
          flyoutForegroundColour: '#e2e8f0',
          flyoutOpacity: 0.95,
          scrollbarColour: '#94a3b8',
          insertionMarkerColour: '#22c55e',
          insertionMarkerOpacity: 0.6,
        },
      } as unknown as Blockly.Theme,
      trashcan: true,
      zoom: { controls: true, wheel: true, startScale: 0.9, maxScale: 2, minScale: 0.4 },
      grid: { spacing: 20, length: 3, colour: '#e2e8f0', snap: true },
      scrollbars: true,
      move: { scrollbars: true, drag: true, wheel: true },
    });

    workspaceRef.current = ws;

    // Load saved XML if any
    if (initialXml) {
      try {
        const dom = Blockly.utils.xml.textToDom(initialXml);
        Blockly.Xml.domToWorkspace(dom, ws);
      } catch { /* ignore */ }
    } else {
      // Load a starter block
      const starterXml = `
        <xml>
          <block type="dns_if_category" x="30" y="30">
            <value name="CATEGORY">
              <block type="dns_category"><field name="CATEGORY">ADULT</field></block>
            </value>
            <statement name="ACTION">
              <block type="dns_block"></block>
            </statement>
          </block>
          <block type="dns_safe_search" x="30" y="160"></block>
        </xml>`;
      const dom = Blockly.utils.xml.textToDom(starterXml);
      Blockly.Xml.domToWorkspace(dom, ws);
    }

    const listener = () => {
      onRulesChange(extractRules(ws));
    };
    ws.addChangeListener(listener);

    return () => {
      ws.removeChangeListener(listener);
      ws.dispose();
      workspaceRef.current = null;
    };
  }, []);

  return (
    <div className="flex flex-col gap-2">
      <p className="text-xs text-gray-500">
        Drag blocks from the left panel to build your DNS filtering logic.
        Changes are applied when you save below.
      </p>
      <div
        ref={containerRef}
        className="w-full rounded-lg border border-gray-200 overflow-hidden"
        style={{ height: '480px' }}
      />
    </div>
  );
}

export function getWorkspaceXml(ws: Blockly.WorkspaceSvg): string {
  return Blockly.Xml.domToText(Blockly.Xml.workspaceToDom(ws));
}

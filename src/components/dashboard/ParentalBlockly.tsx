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
}

// ── Toolbox ───────────────────────────────────────────────────────────────────

const TOOLBOX = {
  kind: 'categoryToolbox',
  contents: [
    {
      kind: 'category',
      name: '🛡️ Rules',
      colour: '210',
      contents: [
        { kind: 'block', type: 'dns_if_category' },
        { kind: 'block', type: 'dns_time_rule' },
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
